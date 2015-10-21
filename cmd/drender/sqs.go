package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"time"

	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/s3"
	"github.com/goamz/goamz/sqs"

	"github.com/ThomasHabets/qpov/dist"
)

func getAuth() aws.Auth {
	return aws.Auth{
		AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}
}

type sqsScheduler struct {
	q *sqs.Queue
}

func (*sqsScheduler) get() (string, string, error) {
	/*	r, err := s.q.ReceiveMessage(1)
		if err != nil {
			return "", "", err
		}
		if len(r.Messages) == 0 {
			return "", "", nil
		}
		return "", r.Messages[0].Body
	*/
	return "", "", fmt.Errorf("not implemented")
}

func (s *sqsScheduler) renew(id string, dur time.Duration) error {
	//_, err := s.q.ChangeMessageVisibility(id, int(dur.Seconds())
	//return err
	return fmt.Errorf("not implemented")
}

func (s *sqsScheduler) done(id string, img, stdout, stderr []byte, j string) error {
	//_, err := s.q.DeleteMessage(&m)
	return fmt.Errorf("not implemented")
}

func newSQSScheduler() scheduler {
	conn := sqs.New(getAuth(), aws.USEast)
	q, err := conn.GetQueue(*queueName)
	if err != nil {
		log.Fatalf("Getting queue: %v", err)
	}
	return &sqsScheduler{q: q}
}

func upload(n int, order *dist.Order) error {
	log.Printf("(%d) Uploading...", n)

	s := s3.New(getAuth(), aws.USEast, nil)

	bucket, destDir, _, _ := dist.S3Parse(order.Destination)
	b := s.Bucket(bucket)

	re := regexp.MustCompile(`\.pov$`)
	image := re.ReplaceAllString(order.File, ".png")

	wd := path.Join(*root, path.Base(order.Package), order.Dir)
	for _, e := range [][]string{
		{"image/png", image},
		{"text/plain", order.File + ".stdout"},
		{"text/plain", order.File + ".stderr"},
		{"text/plain", order.File + infoSuffix},
	} {
		dest := path.Join(destDir, e[1])
		log.Printf("(%d)  s3://%s/%s...", n, bucket, dest)
		acl := s3.ACL("")
		contentType := e[0]
		f, err := os.Open(path.Join(wd, e[1]))
		if err != nil {
			return err
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil {
			return err
		}

		if err := b.PutReader(dest, f, st.Size(), contentType, acl, s3.Options{}); err != nil {
			return err
		}
	}
	return nil
}

func s3Download(n int, order *dist.Order) (string, error) {
	var err error
	s := s3.New(getAuth(), aws.USEast, nil)
	bucket, dir, fn, err := dist.S3Parse(order.Package)
	if err != nil {
		return "", err
	}
	b := s.Bucket(bucket)

	of, err := ioutil.TempFile("", "")
	if err != nil {
		return "", err
	}
	defer os.Remove(of.Name())

	log.Printf("(%d) Downloading %q...", n, order.Package)
	r, err := b.GetReader(path.Join(dir, fn))
	if err != nil {
		return "", fmt.Errorf("getting package: %v", err)
	}
	defer r.Close()
	if _, err := io.Copy(of, r); err != nil {
		return "", fmt.Errorf("downloading package: %v", err)
	}
	if err := of.Close(); err != nil {
		return "", fmt.Errorf("closing package file: %v", err)
	}
	return of.Name(), nil
}
