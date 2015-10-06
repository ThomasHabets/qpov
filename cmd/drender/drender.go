package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"

	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/s3"
	"github.com/goamz/goamz/sqs"

	"github.com/ThomasHabets/qpov/dist"
)

var (
	queueName   = flag.String("queue", "qpov", "Name of SQS queue.")
	povray      = flag.String("povray", "/usr/bin/povray", "Path to POV-Ray.")
	root        = flag.String("wd", "root", "Working directory")
	flush       = flag.Bool("flush", false, "Flush all render jobs.")
	schedtool   = flag.String("schedtool", "/usr/bin/schedtool", "Path to schedtool.")
	concurrency = flag.Int("concurrency", -1, "Run this many povrays in parallel. <0 means set to number of CPUs.")
	idle        = flag.Bool("idle", true, "Use idle priority.")
)

func getAuth() aws.Auth {
	return aws.Auth{
		AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}
}

// verifyPackage downloads and unpacks the package, if needed.
func verifyPackage(n int, order *dist.Order) error {
	wd := path.Join(*root, path.Base(order.Package))

	// Verify working dir.
	{
		st, err := os.Stat(wd)
		if os.IsNotExist(err) {
			// Fine, let's download.
		} else if err != nil {
			return fmt.Errorf("(%d) working dir %q stat failed: %v", n, wd, err)
		} else {
			if !st.IsDir() {
				return fmt.Errorf("(%d) working dir %q exists, but is not a directory", n, wd)
			}
			log.Printf("(%d) Package already downloaded", n)
			return nil
		}
	}

	// Download package.
	var of *os.File
	{
		var err error
		s := s3.New(getAuth(), aws.USEast, nil)
		// TODO: parse out bucket name.
		b := s.Bucket("qpov")

		of, err = ioutil.TempFile("", "")
		if err != nil {
			return err
		}
		defer os.Remove(of.Name())

		// TODO: handle subdirs.
		log.Printf("(%d) Downloading %q...", n, order.Package)
		r, err := b.GetReader(path.Base(order.Package))
		if err != nil {
			return fmt.Errorf("getting package: %v", err)
		}
		defer r.Close()
		if _, err := io.Copy(of, r); err != nil {
			return fmt.Errorf("downloading package: %v", err)
		}
		if err := of.Close(); err != nil {
			return fmt.Errorf("closing package file: %v", err)
		}
	}

	// Unpack.
	{
		err := os.Mkdir(wd, 0700)
		if err != nil && !os.IsExist(err) {
			return fmt.Errorf("creating working dir %q: %v", wd, err)
		}
		log.Printf("(%d) Unpacking %q into %q...", n, order.Package, wd)
		cmd := exec.Command("rar", "x", of.Name())
		cmd.Dir = wd
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

// render renders the order.
func render(n int, order *dist.Order) error {
	log.Printf("(%d) Rendering...", n)
	wd := path.Join(*root, path.Base(order.Package), order.Dir)
	pov := path.Base(order.File)

	var err error
	var bin string
	var args []string
	if *idle {
		bin = *schedtool
		args = append(args, "-D", "-e", *povray)
	} else {
		bin = *povray
	}
	args = append(args, order.Args...)
	args = append(args, pov)
	cmd := exec.Command(bin, args...)
	cmd.Dir = wd
	cmd.Stdout, err = os.Create(path.Join(wd, order.File+".stdout"))
	if err != nil {
		return err
	}
	cmd.Stderr, err = os.Create(path.Join(wd, order.File+".stderr"))
	if err != nil {
		return err
	}
	return cmd.Run()
}

func handle(n int, order *dist.Order) error {
	if err := verifyPackage(n, order); err != nil {
		return err
	}
	if err := render(n, order); err != nil {
		return err
	}
	return nil
}

func handler(n int, q *sqs.Queue) {
	for {
		r, err := q.ReceiveMessage(1)
		if err != nil {
			log.Fatalf("(%d) Receiving message: %v", n, err)
		}
		for _, m := range r.Messages {
			log.Printf("(%d) Got job: %+v", n, m)
			var order dist.Order
			if err := json.Unmarshal([]byte(m.Body), &order); err != nil {
				log.Fatalf("(%d) Failed to unmarshal message %q: %v", n, m.Body, err)
			}
			if !*flush {
				if err := handle(n, &order); err != nil {
					log.Printf("(%d) Failed to handle order %+v: %v", n, order, err)
					continue
				}
			}
			if _, err := q.DeleteMessage(&m); err != nil {
				log.Printf("(%d) Failed to delete message %q", n, m)
			}
		}
	}
}

func main() {
	flag.Parse()
	conn := sqs.New(getAuth(), aws.USEast)
	q, err := conn.GetQueue(*queueName)
	if err != nil {
		log.Fatalf("Getting queue: %v", err)
	}
	if *concurrency <= 0 {
		*concurrency = runtime.NumCPU()
	}

	for c := 0; c < *concurrency; c++ {
		go handler(c, q)
	}
	select {}
}
