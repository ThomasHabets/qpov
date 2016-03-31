package main

// aws.go contains legacy AWS code that eventually should be removed.

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"regexp"
	"sync"

	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/s3"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/ThomasHabets/qpov/dist"
	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

func getAuth() aws.Auth {
	return aws.Auth{
		AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}
}

func awsUpload(leaseID, destination, file string, inImage, inStdout, inStderr []byte, newStats string) error {
	sthree := s3.New(getAuth(), aws.USEast, nil)
	bucket, destDir, _, _ := dist.S3Parse(destination)
	b := sthree.Bucket(bucket)
	destDir = path.Join(destDir, leaseID)
	var wg sync.WaitGroup

	re := regexp.MustCompile(`\.pov$`)
	image := re.ReplaceAllString(file, ".png")

	files := []struct {
		ct   string
		fn   string
		data []byte
	}{
		{"image/png", image, inImage},
		{"text/plain", file + ".stdout", inStdout},
		{"text/plain", file + ".stderr", inStderr},
		{"text/plain", file + infoSuffix, []byte(newStats)},
	}
	errCh := make(chan error, len(files))
	wg.Add(len(files))
	acl := s3.ACL("")
	for _, e := range files {
		e := e
		go func() {
			defer wg.Done()
			if err := b.Put(path.Join(destDir, e.fn), e.data, e.ct, acl, s3.Options{}); err != nil {
				log.Printf("S3 upload of %q error: %v", e.fn, err)
				errCh <- fmt.Errorf("S3 upload error")
			}
		}()
	}
	go func() {
		wg.Wait()
		close(errCh)
	}()
	if err := <-errCh; err != nil {
		return internalError("storing results", "Uploading to S3: %v", err)
	}
	return nil
}

func getOrderDestByLeaseID(id string) (string, string, error) {
	row := db.QueryRow(`SELECT orders.definition FROM orders JOIN leases ON orders.order_id=leases.order_id WHERE lease_id=$1`, id)
	var def string
	if err := row.Scan(&def); err != nil {
		return "", "", err
	}
	var order dist.Order
	if err := json.Unmarshal([]byte(def), &order); err != nil {
		return "", "", err
	}
	return order.Destination, order.File, nil
}

func resultsAWS(ctx context.Context, leaseID, destination, file string, stream pb.Scheduler_ResultServer) error {
	sthree := s3.New(getAuth(), aws.USEast, nil)
	bucket, destDir, _, _ := dist.S3Parse(destination)
	b := sthree.Bucket(bucket)
	destDir2 := path.Join(destDir, leaseID)
	re := regexp.MustCompile(`\.pov$`)
	image := re.ReplaceAllString(file, ".png")
	fn := path.Join(destDir, image)
	fn2 := path.Join(destDir2, image)
	r, err := b.GetReader(fn)
	if err != nil {
		// Look in per-lease dir too.
		r, err = b.GetReader(fn2)
		if err != nil {
			log.Printf("File %q not found on S3: %v", fn, err)
			return grpc.Errorf(codes.NotFound, "file not found")
		}
	}
	defer r.Close()

	writerErr := make(chan error, 1)
	ch := make(chan []byte, 1000)
	// Writer.
	go func() {
		defer func() {
			// Flush the channel, in case there was an error.
			for _ = range ch {
			}
			close(writerErr)
		}()
		writerErr <- func() error {
			for buf := range ch {
				if err := stream.Send(&pb.ResultReply{Data: buf}); err != nil {
					return internalError("failed to stream result", "failed to stream result: %v", err)
				}
			}
			return nil
		}()
	}()

	// Reader.
	if err := func() error {
		defer close(ch)
		for {
			select {
			case err := <-writerErr:
				return err
			case <-ctx.Done(): // TODO: is this needed? Won't stream.Send() fail anyway if context cancels?
				return internalError("timed out", "timed out streaming results %q: %v", image, err)
			default:
			}
			buf := make([]byte, 1024, 1024)
			n, err := r.Read(buf)
			if err == io.EOF {
				break
			}
			if err != nil {
				return internalError("failed to stream result", "failed to stream result %q: %v", image, err)
			}
			ch <- buf[0:n]
		}
		return nil
	}(); err != nil {
		return err
	}
	return <-writerErr
}
