// distributed pov-ray rendering client.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ThomasHabets/qpov/dist"
	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

var (
	addr           = flag.String("addr", ":4900", "Status port to listen to.")
	povray         = flag.String("povray", "/usr/bin/povray", "Path to POV-Ray.")
	refreshTime    = flag.Duration("lease", 30*time.Minute, "Lease time.")
	failWait       = flag.Duration("fail_wait", time.Minute, "Time to pause if rendering fails before trying next order.")
	expiredRenewal = flag.Duration("lease_expired_renewal", time.Minute, "If lease expires, treat lease as living this long.")
	root           = flag.String("wd", "root", "Working directory")
	schedtool      = flag.String("schedtool", "/usr/bin/schedtool", "Path to schedtool.")
	concurrency    = flag.Int("concurrency", 1, "Run this many povrays in parallel. <=0 means set to number of CPUs.")
	idle           = flag.Bool("idle", true, "Use idle priority.")
	comment        = flag.String("comment", "", "Comment to record for stats, for this instance.")
	schedAddr      = flag.String("scheduler", "", "Scheduler address.")

	// DEPRECATED: AWS options.
	queueName = flag.String("queue", "", "Name of SQS queue, if using SQS and S3.")
	flush     = flag.Bool("flush", false, "Flush all render jobs.")

	packageMutex sync.Mutex
	states       []state
)

type scheduler interface {
	get() (string, string, error)
	renew(id string, dur time.Duration) (time.Time, error)
	done(id string, img, stdout, stderr []byte, meta *pb.RenderingMetadata) error
}

type state struct {
	sync.Mutex
	Start time.Time
	Order dist.Order
}

const (
	amazonCloud        = "Amazon"
	googleCloud        = "Google"
	digitalOceanCloud  = "DigitalOcean"
	gceInstanceTypeURL = "http://metadata.google.internal./computeMetadata/v1/instance/machine-type"

	doneRetryTimer = 10 * time.Second

	infoSuffix = ".json"
)

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
	var ofName string
	if strings.HasPrefix(order.Package, "s3://") {
		var err error
		ofName, err = s3Download(n, order)
		if err != nil {
			return err
		}
	} else {
		log.Printf("(%d) Downloading %q...", n, order.Package)
		r, err := http.Get(order.Package)
		if err != nil {
			return err
		}
		defer r.Body.Close()
		of, err := ioutil.TempFile("", "")
		if err != nil {
			return err
		}
		defer os.Remove(of.Name())
		if _, err := io.Copy(of, r.Body); err != nil {
			return fmt.Errorf("downloading package: %v", err)
		}
		if err := of.Close(); err != nil {
			return fmt.Errorf("closing package file: %v", err)
		}
		ofName = of.Name()
	}

	// Unpack.
	if strings.EqualFold(path.Ext(order.Package), ".rar") {
		err := os.Mkdir(wd, 0700)
		if err != nil && !os.IsExist(err) {
			return fmt.Errorf("creating working dir %q: %v", wd, err)
		}
		log.Printf("(%d) Unpacking %q into %q...", n, order.Package, wd)
		cmd := exec.Command("rar", "x", ofName)
		cmd.Dir = wd
		if err := cmd.Run(); err != nil {
			if err := os.RemoveAll(wd); err != nil {
				log.Fatalf("Unrar failed, and deleting working dir failed too, can't recover: %v", err)
			}
			return err
		}
	} else if strings.HasSuffix(strings.ToLower(order.Package), ".tar.gz") {
		err := os.Mkdir(wd, 0700)
		if err != nil && !os.IsExist(err) {
			return fmt.Errorf("creating working dir %q: %v", wd, err)
		}
		log.Printf("(%d) Unpacking %q into %q...", n, order.Package, wd)
		cmd := exec.Command("tar", "xzf", ofName)
		cmd.Dir = wd
		if err := cmd.Run(); err != nil {
			if err := os.RemoveAll(wd); err != nil {
				log.Fatalf("Untar failed, and deleting working dir failed too, can't recover: %v", err)
			}
			return err
		}
	} else {
		return fmt.Errorf("unknown package file type for %q", order.Package)
	}
	return nil
}

// render renders the order.
// produces:
// * png file
// * pov.stderr
// * pov.stdout
// * pov<.infoSuffix>
func render(n int, order *dist.Order) (*pb.RenderingMetadata, error) {
	log.Printf("(%d) Rendering...", n)
	wd := path.Join(*root, path.Base(order.Package), order.Dir)
	pov := order.File

	var err error
	var bin string
	var args []string
	if *idle {
		bin = *schedtool
		args = append(args, "-D", "-n", "19", "-e", *povray)
	} else {
		bin = *povray
	}
	args = append(args, order.Args...)
	args = append(args, "-D", pov)
	cmd := exec.Command(bin, args...)
	cmd.Dir = wd
	cmd.Stdout, err = os.Create(path.Join(wd, order.File+".stdout"))
	if err != nil {
		return nil, err
	}
	cmd.Stderr, err = os.Create(path.Join(wd, order.File+".stderr"))
	if err != nil {
		return nil, err
	}
	startTime := time.Now()
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return makeStats(order, cmd, startTime), nil
}

func getCloud() (string, string) {
	// Check for EC2.
	{
		var b bytes.Buffer
		cmd := exec.Command("ec2metadata", "--instance-type")
		cmd.Stdout = &b
		if cmd.Run() == nil {
			t := strings.TrimSpace(b.String())
			if t != "unavailable" {
				return amazonCloud, t
			}
		}
	}

	// Check for GCE.
	{
		client := http.Client{
			Timeout: 10 * time.Second,
		}
		req, err := http.NewRequest("GET", gceInstanceTypeURL, nil)
		if err != nil {
			log.Printf("Failed to create GCE request: %v", err)
		}
		req.Header.Add("Metadata-Flavor", "Google")
		if resp, err := client.Do(req); err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				re := regexp.MustCompile(`.*/`)
				return googleCloud, re.ReplaceAllString(string(b), "")
			}
		}
	}

	// Check for DigitalOcean.
	{
		// All files must exist.
		do := true
		for _, fn := range []string{"/etc/rc.digitalocean"} {
			if _, err := os.Stat(fn); err != nil {
				do = false
			}
		}
		if do {
			return digitalOceanCloud, "unknown"
		}
	}

	return "", ""
}

// cToStr converts char arrays of size 65 to strings, be they signed or unsigned.
// C strings coming from syscall.Utsname can be anything. Sigh.
func cToStr(cin interface{}) string {
	var c [65]uint8
	switch cin.(type) {
	case [65]uint8:
		c = cin.([65]uint8)
	case [65]int8:
		for n, t := range cin.([65]int8) {
			c[n] = uint8(t)
		}
	default:
		panic(fmt.Errorf("what is cin? %+v", cin))
	}
	s := make([]byte, len(c))
	l := 0
	for ; l < len(c); l++ {
		if c[l] == 0 {
			break
		}
		s[l] = c[l]
	}
	return string(s[0:l])
}

func tv2us(i syscall.Timeval) int64 {
	return int64(i.Sec)*1000000 + int64(i.Usec)
}

func makeStats(order *dist.Order, cmd *exec.Cmd, startTime time.Time) *pb.RenderingMetadata {
	s := &pb.RenderingMetadata{
		Comment: *comment,
	}
	s.Hostname, _ = os.Hostname()

	var u syscall.Utsname
	if err := syscall.Uname(&u); err == nil {
		s.Uname = &pb.Uname{
			Sysname:    cToStr(u.Sysname),
			Nodename:   cToStr(u.Nodename),
			Release:    cToStr(u.Release),
			Version:    cToStr(u.Version),
			Machine:    cToStr(u.Machine),
			Domainname: cToStr(u.Domainname),
		}
	}
	s.NumCpu = int32(runtime.NumCPU())
	s.Version = runtime.Version()
	{
		provider, it := getCloud()
		if provider != "" {
			s.Cloud = &pb.Cloud{
				Provider:     provider,
				InstanceType: it,
			}
		}
	}
	{
		t, _ := ioutil.ReadFile("/proc/cpuinfo")
		if len(t) > 0 {
			s.Cpuinfo = string(t)
		}
	}

	if order != nil {
		t, _ := json.Marshal(order)
		s.OrderString = string(t)
		s.Order = dist.LegacyOrderToOrder(order)
	}
	s.StartMs = startTime.UnixNano() / 1000000
	s.EndMs = time.Now().UnixNano() / 1000000
	if cmd != nil {
		if t, _ := cmd.ProcessState.SysUsage().(*syscall.Rusage); t != nil {
			s.Rusage = &pb.Rusage{
				Utime:    tv2us(t.Utime),
				Stime:    tv2us(t.Stime),
				Maxrss:   int64(t.Maxrss),
				Ixrss:    int64(t.Ixrss),
				Idrss:    int64(t.Idrss),
				Isrss:    int64(t.Isrss),
				Minflt:   int64(t.Minflt),
				Majflt:   int64(t.Majflt),
				Nswap:    int64(t.Nswap),
				Inblock:  int64(t.Inblock),
				Oublock:  int64(t.Oublock),
				Msgsnd:   int64(t.Msgsnd),
				Msgrcv:   int64(t.Msgrcv),
				Nsignals: int64(t.Nsignals),
				Nvcsw:    int64(t.Nvcsw),
				Nivcsw:   int64(t.Nivcsw),
			}
		}
		s.SystemMs = cmd.ProcessState.SystemTime().Nanoseconds() / 1000000
		s.UserMs = cmd.ProcessState.UserTime().Nanoseconds() / 1000000
	}
	return s
}

func handle(n int, order *dist.Order) (*pb.RenderingMetadata, error) {
	// Sanity check order.
	bucket, dir, fn, err := dist.S3Parse(order.Destination)
	if err != nil {
		return nil, fmt.Errorf("destination %q is not an s3 dir path", order.Destination)
	}
	if bucket == "" {
		return nil, fmt.Errorf("destination bucket is empty slash, was %q", order.Destination)
	}
	if dir == "" {
		return nil, fmt.Errorf("refusing to put results in bucket root: %q", order.Destination)
	}
	if fn != "" {
		return nil, fmt.Errorf("destination must end with slash, was %q", order.Destination)
	}

	// Run it.
	if err := verifyPackage(n, order); err != nil {
		return nil, err
	}
	stats, err := render(n, order)
	if err != nil {
		return nil, err
	}

	if *schedAddr == "" {
		// If SQS and not RPC.

		// Write process info.
		{

			wd := path.Join(*root, path.Base(order.Package), order.Dir)
			f, err := os.Create(path.Join(wd, order.File+infoSuffix))
			if err != nil {
				return nil, err
			}
			defer f.Close()
			if str, err := json.Marshal(stats); err != nil {
				return nil, err
			} else {
				if _, err := f.Write(str); err != nil {
					return nil, err
				}
				if err := f.Close(); err != nil {
					return nil, err
				}
			}
		}

		if err := upload(n, order); err != nil {
			return nil, err
		}
	}
	return stats, nil
}

func refresh(q scheduler, id string, refreshCh, doneCh chan struct{}) {
	defer close(doneCh)
	nextTimeout := time.Now().Add(*refreshTime)
	t := time.NewTimer(nextTimeout.Sub(time.Now()) / 2)
	defer t.Stop()
	for {
		select {
		case <-refreshCh:
			return
		case <-t.C:
			n, err := q.renew(id, *refreshTime)
			if err != nil {
				log.Printf("Failed to refresh lease: %v", err)
			} else {
				nextTimeout = n
			}
			now := time.Now()
			if nextTimeout.Before(now) {
				nextTimeout = now.Add(*expiredRenewal)
			}
			t = time.NewTimer(nextTimeout.Sub(now) / 2)
		}
	}
}

// goroutine main() handling rendering.
func handler(n int, q scheduler) {
	for {
		id, encodedOrder, err := q.get()
		if err != nil {
			log.Printf("(%d) Getting order: %v", n, err)
			time.Sleep(1 * time.Minute)
			continue
		}
		if id == "" {
			log.Printf("(%d) Nothing to do...", n)
			time.Sleep(10 * time.Second)
			continue
		}
		refreshChan := make(chan struct{})
		doneChan := make(chan struct{})
		go refresh(q, id, refreshChan, doneChan)
		var order dist.Order
		var meta *pb.RenderingMetadata
		ok := func() bool {
			defer func() {
				states[n].Lock()
				defer states[n].Unlock()
				states[n].Order = dist.Order{}
				<-doneChan
			}()
			defer close(refreshChan)
			log.Printf("(%d) Got job: %q: %q", n, id, encodedOrder)
			if err := json.Unmarshal([]byte(encodedOrder), &order); err != nil {
				log.Printf("(%d) Failed to unmarshal message %q: %v", n, encodedOrder, err)
				return false
			}
			states[n].Lock()
			states[n].Start = time.Now()
			states[n].Order = order
			states[n].Unlock()
			if !*flush {
				meta, err = handle(n, &order)
				if err != nil {
					log.Printf("(%d) Failed to handle order %+v: %v", n, order, err)
					return false
				}
			}
			return true
		}()
		if ok {
			base := path.Join(*root, path.Base(order.Package), order.Dir)
			re := regexp.MustCompile(`\.pov$`)
			img, err := ioutil.ReadFile(path.Join(base, re.ReplaceAllString(order.File, ".png")))
			if err != nil {
				log.Printf("(%d) Failed to read output png: %v", err)
				continue
			}
			stdout, err := ioutil.ReadFile(path.Join(base, order.File+".stdout"))
			if err != nil {
				log.Printf("(%d) Failed to read stdout: %v", err)
				continue
			}
			stderr, err := ioutil.ReadFile(path.Join(base, order.File+".stderr"))
			if err != nil {
				log.Printf("(%d) Failed to read stderr: %v", err)
				continue
			}
			for {
				// Retry forever. We don't want to lose work.
				err := q.done(id, img, stdout, stderr, meta)
				if err == nil {
					log.Printf("(%d) Done", n)
					break
				}
				log.Printf("(%d) Failed to delete message %q. Retrying.", n, id)
				time.Sleep(doneRetryTimer)
			}
		} else {
			log.Printf("(%d) Order failed. Waiting %v", n, *failWait)
			time.Sleep(*failWait)
		}
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	st := makeStats(nil, nil, time.Now())
	t := template.Must(template.New("root").Parse(`
<html>
  <head>
    <title>drender on {{.Stats.Hostname}}</title>
    <style>
#activity td {
  border: 1px black solid;
}
    </style>
  </head>
  <body>
    <h1>drender on {{.Stats.Hostname}}</h1>
    <h2>System</h2>
    <table>
      <tr><th>NumCPU</th><td>{{.Stats.NumCpu}}</td></tr>
    </table>
    <h2>Activity</h2>
    <table id="activity">
      <tr>
        <th>Start</th>
        <th>Package</th>
        <th>Dir</th>
        <th>File</th>
        <th>Destination</th>
        <th>Args</th>
      </tr>
      {{range .States}}
        <tr>
          <td>{{.Start}}</td>
          <td>{{.Order.Package}}</td>
          <td>{{.Order.Dir}}</td>
          <td>{{.Order.File}}</td>
          <td>{{.Order.Destination}}</td>
          <td>{{.Order.Args}}</td>
        </tr>
      {{end}}
    </table>
  </body>
</html>`))
	for n := range states {
		states[n].Lock()
		defer states[n].Unlock()
	}
	if err := t.Execute(w, struct {
		Stats  *pb.RenderingMetadata
		States []state
	}{
		Stats:  st,
		States: states,
	}); err != nil {
		log.Printf("Template rendering error: %v", err)
	}
}

func main() {
	flag.Parse()
	if len(flag.Args()) != 0 {
		log.Fatalf("Got extra args on cmdline: %q", flag.Args())
	}
	if *queueName != "" {
		log.Fatalf("SQS code untested and will eventually be removed. Please use -scheduler.")
	}

	log.Printf("Starting up...")

	var s scheduler
	var err error
	s, err = newRPCScheduler(*schedAddr)
	if err != nil {
		log.Fatalf("Failed to set up scheduler: %v", err)
	}
	if *concurrency <= 0 {
		*concurrency = runtime.NumCPU()
	}

	states = make([]state, *concurrency, *concurrency)
	for c := 0; c < *concurrency; c++ {
		go handler(c, s)
	}
	http.HandleFunc("/", handleRoot)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
