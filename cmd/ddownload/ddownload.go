// Download all rendered files into a zipfile.
package main

import (
	"archive/zip"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path"

	log "github.com/sirupsen/logrus"

	//"github.com/ThomasHabets/qpov/pkg/dist"
	pb "github.com/ThomasHabets/qpov/pkg/dist/qpov"
	"github.com/ThomasHabets/qpov/pkg/dist/rpcclient"
)

var (
	out     = flag.String("out", "", "Output zipfile.")
	batchID = flag.String("batch", "", "Batch to download.")
	dryRun  = flag.Bool("dry_run", false, "Put fake data in zip file instead of downloading from cloud.")

	schedAddr = flag.String("scheduler", "qpov.retrofitta.se:9999", "Scheduler address.")
	sched     *rpcclient.RPCScheduler
)

func replaceExt(fn, n string) string {
	return fn[0:len(fn)-len(path.Ext(fn))] + n
}

func getLeases(ctx context.Context) ([]*pb.Lease, error) {
	stream, err := sched.Client.Leases(ctx, &pb.LeasesRequest{
		Done:  true,
		Order: true,
		Batch: *batchID,
	})
	if err != nil {
		return nil, err
	}
	var ret []*pb.Lease
	for {
		r, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		ret = append(ret, r.Lease)
	}
	return ret, nil
}

func ddownload(ctx context.Context, o *zip.Writer) error {
	leases, err := getLeases(ctx)
	if err != nil {
		return err
	}
	leases = leases

	for _, lease := range leases {
		fmt.Printf("File: %q\n", lease.Order.File)
		stream, err := sched.Client.Result(ctx, &pb.ResultRequest{
			LeaseId: lease.LeaseId,
			Data:    true,
		})
		if err != nil {
			return err
		}

		w, err := o.Create(replaceExt(path.Base(lease.Order.File), ".png"))
		if err != nil {
			return err
		}
		for {
			r, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if _, err := w.Write(r.Data); err != nil {
				return err
			}
		}
	}
	return nil
}

func main() {
	ctx := context.Background()
	log.Infof("Running ddownload")
	flag.Parse()
	if flag.NArg() > 0 {
		log.Fatalf("Got extra args on cmdline: %q", flag.Args())
	}

	if *out == "" {
		log.Fatalf("-out is mandatory")
	}
	if *batchID == "" {
		log.Fatalf("-batch is mandatory")
	}

	var err error
	sched, err = rpcclient.NewScheduler(ctx, *schedAddr)
	if err != nil {
		log.Fatalf("Failed to connect to %q: %v", *schedAddr, err)
	}
	defer sched.Close()

	if _, err := os.Lstat(*out); err == nil {
		log.Fatalf("%q already exists", *out)
	}

	fo, err := os.Create(*out)
	if err != nil {
		log.Fatalf("Failed to create -out %q: %v", *out, err)
	}
	defer func() {
		if err := fo.Close(); err != nil {
			log.Fatalf("Failed to close %q: %v", *out, err)
		}
	}()
	fz := zip.NewWriter(fo)
	defer func() {
		if err := fz.Close(); err != nil {
			log.Fatalf("Failed to close zip writer for %q: %v", *out, err)
		}
	}()

	if err := ddownload(ctx, fz); err != nil {
		log.Fatal(err)
	}
}
