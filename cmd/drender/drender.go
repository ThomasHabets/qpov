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
	"regexp"
	"runtime"
	"sync"
	"time"

	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/s3"
	"github.com/goamz/goamz/sqs"

	"github.com/ThomasHabets/qpov/dist"
)

var (
	queueName   = flag.String("queue", "", "Name of SQS queue.")
	povray      = flag.String("povray", "/usr/bin/povray", "Path to POV-Ray.")
	refreshTime = flag.Duration("lease", 30*time.Second, "Lease time.")
	root        = flag.String("wd", "root", "Working directory")
	flush       = flag.Bool("flush", false, "Flush all render jobs.")
	schedtool   = flag.String("schedtool", "/usr/bin/schedtool", "Path to schedtool.")
	concurrency = flag.Int("concurrency", 1, "Run this many povrays in parallel. <=0 means set to number of CPUs.")
	idle        = flag.Bool("idle", true, "Use idle priority.")

	packageMutex sync.Mutex
)

func getAuth() aws.Auth {
	return aws.Auth{
		AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}
}

// verifyPackage downloads and unpacks the package, if needed.
func verifyPackage(n int, order *dist.Order) error {
	// Don't re-enter, to make sure we don't start the same download twice.
	packageMutex.Lock()
	defer packageMutex.Unlock()

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
		bucket, dir, fn, err := s3Parse(order.Package)
		if err != nil {
			return err
		}
		b := s.Bucket(bucket)

		of, err = ioutil.TempFile("", "")
		if err != nil {
			return err
		}
		defer os.Remove(of.Name())

		log.Printf("(%d) Downloading %q...", n, order.Package)
		r, err := b.GetReader(path.Join(dir, fn))
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
// produces:
// * png file
// * pov.stderr
// * pov.stdout
// * pov.info
func render(n int, order *dist.Order) error {
	log.Printf("(%d) Rendering...", n)
	wd := path.Join(*root, path.Base(order.Package), order.Dir)
	pov := order.File

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
	args = append(args, "-D", pov)
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
	if err := cmd.Run(); err != nil {
		return err
	}

	// Write process info.
	{
		f, err := os.Create(path.Join(wd, order.File+".info"))
		if err != nil {
			return err
		}
		defer f.Close()
		s := cmd.ProcessState
		fmt.Fprintf(f, "system %f\n", s.SystemTime().Seconds())
		fmt.Fprintf(f, "user %f\n", s.UserTime().Seconds())
	}
	return nil
}

func upload(n int, order *dist.Order) error {
	log.Printf("(%d) Uploading...", n)

	s := s3.New(getAuth(), aws.USEast, nil)

	bucket, destDir, _, _ := s3Parse(order.Destination)
	b := s.Bucket(bucket)

	re := regexp.MustCompile(`\.pov$`)
	image := re.ReplaceAllString(order.File, ".png")

	wd := path.Join(*root, path.Base(order.Package), order.Dir)
	for _, e := range [][]string{
		{"image/png", image},
		{"text/plain", order.File + ".stdout"},
		{"text/plain", order.File + ".stderr"},
		{"text/plain", order.File + ".info"},
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

// Return bucket, dir, fn.
func s3Parse(s string) (string, string, string, error) {
	r := `^s3://([^/]+)/(.*)(?:/(.*))?$`
	re := regexp.MustCompile(r)
	m := re.FindStringSubmatch(s)
	if len(m) != 4 {
		return "", "", "", fmt.Errorf("%q does not match %q", s, r)
	}
	return m[1], m[2], m[3], nil
}

func handle(n int, order *dist.Order) error {
	// Sanity check order.
	bucket, dir, fn, err := s3Parse(order.Destination)
	if err != nil {
		return fmt.Errorf("destination %q is not an s3 dir path", order.Destination)
	}
	if bucket == "" {
		return fmt.Errorf("destination bucket is empty slash, was %q", order.Destination)
	}
	if dir == "" {
		return fmt.Errorf("refusing to put results in bucket root: %q", order.Destination)
	}
	if fn != "" {
		return fmt.Errorf("destination must end with slash, was %q", order.Destination)
	}

	// Run it.
	if err := verifyPackage(n, order); err != nil {
		return err
	}
	if err := render(n, order); err != nil {
		return err
	}
	if err := upload(n, order); err != nil {
		return err
	}
	return nil
}

func refresh(q *sqs.Queue, m *sqs.Message, refreshCh, doneCh chan struct{}) {
	defer close(doneCh)
	t := time.NewTicker(*refreshTime)
	defer t.Stop()
	for {
		select {
		case <-refreshCh:
			return
		case <-t.C:
			if _, err := q.ChangeMessageVisibility(m, int(refreshTime.Seconds())*2); err != nil {
				log.Printf("Failed to refresh message: %v", err)
			}
		}
	}
}

func handler(n int, q *sqs.Queue) {
	for {
		r, err := q.ReceiveMessage(1)
		if err != nil {
			log.Fatalf("(%d) Receiving message: %v", n, err)
		}
		if len(r.Messages) == 0 {
			log.Printf("(%d) Nothing to do...", n)
			time.Sleep(10 * time.Second)
			continue
		}
		m := r.Messages[0]
		refreshChan := make(chan struct{})
		doneChan := make(chan struct{})
		go refresh(q, &m, refreshChan, doneChan)
		ok := func() bool {
			defer func() {
				<-doneChan
			}()
			defer close(refreshChan)
			log.Printf("(%d) Got job: %+v", n, m)
			var order dist.Order
			if err := json.Unmarshal([]byte(m.Body), &order); err != nil {
				log.Printf("(%d) Failed to unmarshal message %q: %v", n, m.Body, err)
				return false
			}
			if !*flush {
				if err := handle(n, &order); err != nil {
					log.Printf("(%d) Failed to handle order %+v: %v", n, order, err)
					return false
				}
			}
			return true
		}()
		if ok {
			if _, err := q.DeleteMessage(&m); err != nil {
				log.Printf("(%d) Failed to delete message %q", n, m)
			} else {
				log.Printf("(%d) Done", n)
			}
		}
	}
}

func main() {
	flag.Parse()
	log.Printf("Starting up...")
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
