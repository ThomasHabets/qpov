package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/ThomasHabets/qpov/pkg/dist"
	pb "github.com/ThomasHabets/qpov/pkg/dist/qpov"
)

var (
	schedAddr = flag.String("scheduler", "", "Scheduler address.")

	commands = map[string]func([]string){
		"leases": cmdLeases,
		"orders": cmdOrders,
		"stats":  cmdStats,
		"add":    cmdAdd,
	}
)

type scheduler interface {
	add(order, batchID string) error
	close()
}

type Range struct {
	From, To, Skip int
}

func (r *Range) Set(s string) error {
	res := `^(\d+)-(\d+)(?:\+(\d+))?$`
	re := regexp.MustCompile(res)
	m := re.FindStringSubmatch(s)
	if len(m) != 4 {
		return fmt.Errorf("-frames must match regex %q. Was %q", res, s)
	}
	var err error
	if r.From, err = strconv.Atoi(m[1]); err != nil {
		return err
	}
	if r.To, err = strconv.Atoi(m[2]); err != nil {
		return err
	}
	if m[3] == "" {
		r.Skip = 1
	} else {
		if r.Skip, err = strconv.Atoi(m[3]); err != nil {
			return err
		}
	}
	if r.Skip < 1 {
		fmt.Errorf("skip must be at least 1, was %d", r.Skip)
	}
	return nil
}
func (r *Range) String() string {
	return fmt.Sprintf("%d-%d+%d", r.From, r.To, r.Skip)
}

func ms2Time(t int64) time.Time {
	return time.Unix(t/1000, 1000000*(t%1000))
}

func roundSecondD(t time.Duration) time.Duration {
	return time.Duration((int64(t) / 1000000000) * 1000000000)
}

func cmdLeases(args []string) {
	fs := flag.NewFlagSet("leases", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] leases [options]\n", os.Args[0])
		fs.PrintDefaults()
	}
	done := fs.Bool("done", false, "List completed leases, as opposed to active.")
	fs.Parse(args)

	q, err := newRPCScheduler(*schedAddr)
	if err != nil {
		log.Fatalf("Connecting to scheduler %q: %v", *schedAddr, err)
	}
	defer q.close()
	stream, err := q.client.Leases(context.Background(), &pb.LeasesRequest{
		Done: *done,
	})
	if err != nil {
		log.Fatalf("Listing leases: %v", err)
	}
	f := "%36s %36s %5s %16s %16s %15s\n"
	if *done {
		fmt.Printf(f, "Order ID", "Lease ID", "User", "Created", "Done", "Time")
	} else {
		fmt.Printf(f, "Order ID", "Lease ID", "User", "Lifetime", "Updated", "Expires")
	}
	for {
		r, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("Listing leases: %v", err)
		}
		l := r.Lease
		if *done {
			tf := "2006-01-02 15:04"
			fmt.Printf(f, l.OrderId, l.LeaseId, fmt.Sprint(l.UserId),
				ms2Time(l.CreatedMs).Format(tf),
				ms2Time(l.UpdatedMs).Format(tf),
				roundSecondD(ms2Time(l.UpdatedMs).Sub(ms2Time(l.CreatedMs))),
			)
		} else {
			fmt.Printf(f, l.OrderId, l.LeaseId, fmt.Sprint(l.UserId),
				roundSecondD(time.Since(ms2Time(l.CreatedMs))),
				roundSecondD(time.Since(ms2Time(l.UpdatedMs))),
				roundSecondD(ms2Time(l.ExpiresMs).Sub(time.Now())),
			)
		}
	}
}

func cmdOrders(args []string) {
	fs := flag.NewFlagSet("orders", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] orders [options]\n", os.Args[0])
		fs.PrintDefaults()
	}
	done := fs.Bool("done", false, "List completed orders.")
	active := fs.Bool("active", true, "List active orders.")
	unstarted := fs.Bool("unstarted", true, "List unstarted orders.")
	fs.Parse(args)

	q, err := newRPCScheduler(*schedAddr)
	if err != nil {
		log.Fatalf("Connecting to scheduler %q: %v", *schedAddr, err)
	}
	defer q.close()
	stream, err := q.client.Orders(context.Background(), &pb.OrdersRequest{
		Done:      *done,
		Active:    *active,
		Unstarted: *unstarted,
	})
	if err != nil {
		log.Fatalf("Listing orders: %v", err)
	}
	f := "%36s %10s %10s\n"
	fmt.Printf(f, "Order ID", "Active", "Done")
	for {
		r, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("Listing orders: %v", err)
		}
		l := r.Order
		fmt.Printf(f, l.OrderId, fmt.Sprint(l.Active), fmt.Sprint(l.Done))
	}
}

func cmdStats(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] stats [options]\n", os.Args[0])
		fs.PrintDefaults()
	}
	fs.Parse(args)

	q, err := newRPCScheduler(*schedAddr)
	if err != nil {
		log.Fatalf("Connecting to scheduler %q: %v", *schedAddr, err)
	}
	defer q.close()
	res, err := q.client.Stats(context.Background(), &pb.StatsRequest{
		SchedulingStats: true,
	})
	if err != nil {
		log.Fatalf("Getting stats: %v", err)
	}
	fmt.Printf(`-- Scheduling stats --
Orders:    Total: %10d   Active: %10d    Done: %10d
Leases:    Total: %10d   Active: %10d    Done: %10d
`,
		res.SchedulingStats.Orders, res.SchedulingStats.ActiveOrders, res.SchedulingStats.DoneOrders,
		res.SchedulingStats.Leases, res.SchedulingStats.ActiveLeases, res.SchedulingStats.DoneLeases,
	)
}

func cmdAdd(args []string) {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] add [options] <povray args...>\n", os.Args[0])
		fs.PrintDefaults()
	}
	pkg := fs.String("package", "", "URL path to rar/tgz file containing all resources.")
	batch := fs.String("batch", "", "Batch this belongs to.")
	dir := fs.String("dir", "", "Directory in package to use as CWD.")
	file := fs.String("file", "", "POV file to render.")
	dryRun := fs.Bool("dry_run", false, "Don't actually enqueue.")
	var frames Range
	fs.Var(&frames, "frames", "Order many frames to be rendered. In format '1-10' or '1-10+2' for only doing odd numbered frames. Use fmt string in '-file'")
	fs.Parse(args)

	if *pkg == "" {
		log.Fatalf("Must supply -package")
	}
	if *file == "" {
		log.Fatalf("Must supply -file")
	}

	// For backwards compatability with clients that try to parse this destination path.
	const dst = "s3://dummy/dummy/dummy/"

	var q scheduler
	var err error
	log.Printf("Connecting...")
	q, err = newRPCScheduler(*schedAddr)
	if err != nil {
		log.Fatalf("Connecting to scheduler %q: %v", *schedAddr, err)
	}
	log.Printf("... connected")
	defer q.close()

	if frames.Skip == 0 {
		// Just one.
		order, err := json.Marshal(&dist.Order{
			Package:     *pkg,
			Dir:         *dir,
			File:        *file,
			Destination: dst,
			Args:        fs.Args(),
			//Args:        []string{"+Q11", "+A0.3", "+R4", "+W3840", "+H2160"},
			//Args: []string{"+W320", "+H240"},
		})
		if err != nil {
			log.Fatalf("JSON-marshaling order: %v", err)
		}
		if *dryRun {
			log.Printf("Would have scheduled %v", string(order))
		} else {
			if err := q.add(string(order), *batch); err != nil {
				log.Fatalf("Failed to enqueue: %v", err)
			}
		}
	} else {
		// Ugly way of counting, but I'm tired.
		c := 0
		for i := frames.From; i <= frames.To; i += frames.Skip {
			c++
		}
		fmt.Printf(`Will schedule %d render jobs of this form:
  Package:     %q
  Dir:         %q
  File:        %q (becomes e.g. %q)
  Destination: %q
  Args:        %q

OK (y/N)?
`, c, *pkg, *dir, *file, fmt.Sprintf(*file, 1), dst, fs.Args())

		var yn string
		fmt.Scanln(&yn)
		if !strings.EqualFold(yn, "y") {
			return
		}
		for i := frames.From; i <= frames.To; i += frames.Skip {
			order, err := json.Marshal(&dist.Order{
				Package:     *pkg,
				Dir:         *dir,
				File:        fmt.Sprintf(*file, i),
				Destination: dst,
				Args:        fs.Args(),
				//Args:        []string{"+Q11", "+A0.3", "+R4", "+W3840", "+H2160"},
				//Args: []string{"+W320", "+H240"},
			})
			if err != nil {
				log.Fatalf("JSON-marshaling order: %v", err)
			}
			if *dryRun {
				log.Printf("Would have scheduled %v", string(order))
			} else {
				if err := q.add(string(order), *batch); err != nil {
					log.Fatalf("Failed to enqueue: %v", err)
				}
			}
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [global options] command [options]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Commands:\n")
	for k := range commands {
		fmt.Fprintf(os.Stderr, "  %s\n", k)
	}
	fmt.Fprintf(os.Stderr, "Global options:\n")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if len(flag.Args()) == 0 {
		usage()
		log.Fatalf("Must supply command.")
	}

	if *schedAddr == "" {
		log.Fatalf("Must supply -scheduler")
	}

	f, found := commands[flag.Arg(0)]
	if !found {
		log.Fatalf("Unknown command %q", flag.Arg(0))
	}
	f(flag.Args()[1:])
}
