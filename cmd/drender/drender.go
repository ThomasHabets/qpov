// distributed pov-ray rendering client.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
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

	log "github.com/sirupsen/logrus"

	"github.com/ThomasHabets/qpov/pkg/dist"
	pb "github.com/ThomasHabets/qpov/pkg/dist/qpov"
	"github.com/ThomasHabets/qpov/pkg/dist/rpcclient"
)

var (
	addr           = flag.String("addr", ":4900", "Status port to listen to.")
	povray         = flag.String("povray", "/usr/bin/povray", "Path to POV-Ray.")
	refreshTime    = flag.Duration("lease", 30*time.Minute, "Lease time.")
	failWait       = flag.Duration("fail_wait", time.Minute, "Time to pause if rendering fails before trying next order.")
	expiredRenewal = flag.Duration("lease_expired_renewal", time.Minute, "If lease expires, treat lease as living this long.")
	root           = flag.String("wd", "wd", "Working directory")
	schedtool      = flag.String("schedtool", "/usr/bin/schedtool", "Path to schedtool.")
	concurrency    = flag.Int("concurrency", 1, "Run this many povrays in parallel. <=0 means set to number of CPUs.")
	idle           = flag.Bool("idle", true, "Use idle priority.")
	comment        = flag.String("comment", "", "Comment to record for stats, for this instance.")
	schedAddr      = flag.String("scheduler", "qpov.retrofitta.se:9999", "Scheduler address.")

	flush = flag.Bool("flush", false, "Flush all render jobs.")

	packageMutex sync.Mutex
	states       []state
)

type scheduler interface {
	Get(ctx context.Context) (string, string, error)
	Renew(ctx context.Context, id string, dur time.Duration) (time.Time, error)
	Done(ctx context.Context, id string, img, stdout, stderr []byte, meta *pb.RenderingMetadata) error
}

type state struct {
	sync.Mutex
	Start time.Time
	Order dist.Order
}

const (
	amazonCloud       = "Amazon"
	googleCloud       = "Google"
	digitalOceanCloud = "DigitalOcean"

	// These should be the same, but the latter doesn't rely on DNS.
	//gceInstanceTypeURL = "http://metadata.google.internal./computeMetadata/v1/instance/machine-type"
	gceInstanceTypeURL = "http://169.254.169.254/computeMetadata/v1/instance/machine-type"

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
			log.Infof("(%d) Package already downloaded", n)
			return nil
		}
	}

	// Download package.
	var ofName string
	{
		log.Infof("(%d) Downloading %q...", n, order.Package)
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
		log.Infof("(%d) Unpacking %q into %q...", n, order.Package, wd)
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
		log.Infof("(%d) Unpacking %q into %q...", n, order.Package, wd)
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
	log.Infof("(%d) Rendering...", n)
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
			log.Errorf("Failed to create GCE request: %v", err)
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
	if order.Destination != "" {
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
	}

	// Run it.
	if err := verifyPackage(n, order); err != nil {
		return nil, err
	}
	stats, err := render(n, order)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

func refresh(ctx context.Context, q scheduler, id string, refreshCh, doneCh chan struct{}) {
	defer close(doneCh)
	nextTimeout := time.Now().Add(*refreshTime)
	t := time.NewTimer(nextTimeout.Sub(time.Now()) / 2)
	defer t.Stop()
	for {
		select {
		case <-refreshCh:
			return
		case <-t.C:
			n, err := q.Renew(ctx, id, *refreshTime)
			if err != nil {
				log.Warningf("Failed to refresh lease: %v", err)
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
func handler(ctx context.Context, n int, q scheduler) {
	for {
		id, encodedOrder, err := q.Get(ctx)
		if err != nil {
			log.Infof("(%d) Getting order: %v", n, err)
			time.Sleep(1 * time.Minute)
			continue
		}
		if id == "" {
			log.Infof("(%d) Nothing to do...", n)
			time.Sleep(10 * time.Second)
			continue
		}
		refreshChan := make(chan struct{})
		doneChan := make(chan struct{})
		go refresh(ctx, q, id, refreshChan, doneChan)
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
			log.Infof("(%d) Got job: %q: %q", n, id, encodedOrder)
			if err := json.Unmarshal([]byte(encodedOrder), &order); err != nil {
				log.Errorf("(%d) Failed to unmarshal message %q: %v", n, encodedOrder, err)
				return false
			}
			states[n].Lock()
			states[n].Start = time.Now()
			states[n].Order = order
			states[n].Unlock()
			meta, err = handle(n, &order)
			if err != nil {
				log.Errorf("(%d) Failed to handle order %+v: %v", n, order, err)
				return false
			}
			return true
		}()
		if ok {
			base := path.Join(*root, path.Base(order.Package), order.Dir)
			re := regexp.MustCompile(`\.pov$`)
			img, err := ioutil.ReadFile(path.Join(base, re.ReplaceAllString(order.File, ".png")))
			if err != nil {
				log.Errorf("(%d) Failed to read output png: %v", n, err)
				continue
			}
			stdout, err := ioutil.ReadFile(path.Join(base, order.File+".stdout"))
			if err != nil {
				log.Errorf("(%d) Failed to read stdout: %v", n, err)
				continue
			}
			stderr, err := ioutil.ReadFile(path.Join(base, order.File+".stderr"))
			if err != nil {
				log.Errorf("(%d) Failed to read stderr: %v", n, err)
				continue
			}
			for {
				// Retry forever. We don't want to lose work.
				err := q.Done(ctx, id, img, stdout, stderr, meta)
				if err == nil {
					log.Infof("(%d) Done", n)
					break
				}
				log.Errorf("(%d) Failed to delete message %q. Retrying.", n, id)
				time.Sleep(doneRetryTimer)
			}
		} else {
			log.Errorf("(%d) Order failed. Waiting %v", n, *failWait)
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
		log.Errorf("Template rendering error: %v", err)
	}
}

func main() {
	flag.Parse()
	if len(flag.Args()) != 0 {
		log.Fatalf("Got extra args on cmdline: %q", flag.Args())
	}

	log.Infof("Starting up...")
	ctx := context.Background()
	var s scheduler
	var err error
	s, err = rpcclient.NewScheduler(ctx, *schedAddr)
	if err != nil {
		log.Fatalf("Failed to set up scheduler: %v", err)
	}
	if *concurrency <= 0 {
		*concurrency = runtime.NumCPU()
	}

	states = make([]state, *concurrency, *concurrency)
	for c := 0; c < *concurrency; c++ {
		go handler(ctx, c, s)
	}
	http.HandleFunc("/", handleRoot)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
