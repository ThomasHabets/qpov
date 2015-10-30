package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/ThomasHabets/qpov/dist"
)

var (
	queueName = flag.String("queue", "", "Name of SQS queue.")
	pkg       = flag.String("package", "", "S3 path to rar file containing all resources.")
	dir       = flag.String("dir", "", "Directory in package to use as CWD.")
	file      = flag.String("file", "", "POV file to render.")
	dst       = flag.String("destination", "", "S3 directory to store results in.")
	dryRun    = flag.Bool("dry_run", false, "Don't actually enqueue.")
	schedAddr = flag.String("scheduler", "", "Scheduler address.")

	frames Range
)

func init() {
	flag.Var(&frames, "frames", "Order many frames to be rendered. In format '1-10' or '1-10+2' for only doing odd numbered frames.")
}

type scheduler interface {
	add(string) error
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

func main() {
	flag.Parse()

	if *schedAddr == "" && *queueName == "" {
		log.Fatalf("Must supply -queue or -scheduler")
	}
	if *pkg == "" {
		log.Fatalf("Must supply -package")
	}
	if *file == "" {
		log.Fatalf("Must supply -file")
	}
	if *dst == "" {
		log.Fatalf("Must supply -destination")
	}

	var q scheduler
	var err error
	q, err = newRPCScheduler(*schedAddr)
	if err != nil {
		log.Fatalf("Connecting to scheduler %q: %v", *schedAddr, err)
	}
	defer q.close()

	if frames.Skip == 0 {
		// Just one.
		order, err := json.Marshal(&dist.Order{
			Package:     *pkg,
			Dir:         *dir,
			File:        *file,
			Destination: *dst,
			Args:        flag.Args(),
			//Args:        []string{"+Q11", "+A0.3", "+R4", "+W3840", "+H2160"},
			//Args: []string{"+W320", "+H240"},
		})
		if err != nil {
			log.Fatalf("JSON-marshaling order: %v", err)
		}
		if *dryRun {
			log.Printf("Would have scheduled %v", string(order))
		} else {
			if err := q.add(string(order)); err != nil {
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
`, c, *pkg, *dir, *file, fmt.Sprintf(*file, 1), *dst, flag.Args())

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
				Destination: *dst,
				Args:        flag.Args(),
				//Args:        []string{"+Q11", "+A0.3", "+R4", "+W3840", "+H2160"},
				//Args: []string{"+W320", "+H240"},
			})
			if err != nil {
				log.Fatalf("JSON-marshaling order: %v", err)
			}
			if *dryRun {
				log.Printf("Would have scheduled %v", string(order))
			} else {
				if err := q.add(string(order)); err != nil {
					log.Fatalf("Failed to enqueue: %v", err)
				}
			}
		}
	}
}
