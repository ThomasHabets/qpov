package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	_ "github.com/lib/pq"

	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

var (
	db *sql.DB
	// TODO: Ask the scheduler for the leases instead of getting them from the DB directly.
	dbConnect = flag.String("db", "", "Database connect string.")
)

type event struct {
	time    time.Time
	lease   int
	cpuTime int64
}

type byTime []event

func (a byTime) Len() int           { return len(a) }
func (a byTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byTime) Less(i, j int) bool { return a[i].time.Before(a[j].time) }

func mkstats() error {
	rows, err := db.Query(`
SELECT metadata
FROM leases
WHERE done=TRUE
AND metadata IS NOT NULL
`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var cpuUS, realMS int64
	var bogoI float64
	bogoRE := regexp.MustCompile(`(?m)^bogomips\s*:\s*([\d.]+)$`)

	// Deltas.
	var events []event
	for rows.Next() {
		var meta sql.NullString
		if err := rows.Scan(&meta); err != nil {
			return err
		}
		var stats pb.RenderingMetadata
		if err := json.Unmarshal([]byte(meta.String), &stats); err != nil {
			log.Printf("Parsing %q: %v", meta.String, err)
			continue
		}
		sbogo := bogoRE.FindAllStringSubmatch(stats.Cpuinfo, -1)
		var bogo float64
		// Get average bogomips.
		if len(sbogo) > 0 {
			var t float64
			var n int
			for _, b := range sbogo {
				bogo, err = strconv.ParseFloat(b[1], 64)
				if err != nil {
					log.Printf("Failed to parse bogomips %q: %v", b[1], err)
				} else {
					n++
					t += bogo
				}
			}
			if n > 0 {
				bogo = t / float64(n)
			}
			if n != int(stats.NumCpu) {
				log.Printf("#bogomips != numcpu: %d != %d", n, stats.NumCpu)
			}
		}
		bogoI += float64(stats.Rusage.Utime+stats.Rusage.Stime) * bogo
		cpuUS += stats.Rusage.Utime + stats.Rusage.Stime
		realMS += stats.EndMs - stats.StartMs
		events = append(events,
			event{
				time:  time.Unix(stats.StartMs/1000, stats.StartMs%1000*1000000),
				lease: 1,
			},
			event{
				time:  time.Unix(stats.EndMs/1000, stats.EndMs%1000*1000000),
				lease: -1,
			})
	}
	if rows.Err() != nil {
		return err
	}
	fmt.Printf("CPU time:     %40s\n", formatDuration(time.Duration(cpuUS*1000)))
	fmt.Printf("Machine time: %40s\n", formatDuration(time.Duration(realMS*1000000)))
	fmt.Printf("BogoI:        %40s\n", formatFloat(bogoI))

	// BogoSpeed of various machines.
	bogoIPS := []struct {
		name      string
		bogoSpeed float64
	}{
		{"Raspberry Pi 2", 4 * 64.0 * 1e6},
		{"Raspberry Pi 3", 4 * 76.80 * 1e6},
		{"Viking", 32 * 4800 * 1e6},
		{"EC2 c4.8xlarge", 36 * 5800 * 1e6},
	}
	fmt.Printf("Machine equivalents:\n")
	for _, b := range bogoIPS {
		fmt.Printf("    %-20s:        %v\n", b.name, formatDuration(time.Duration(1e9*bogoI/b.bogoSpeed)))
	}

	sort.Sort(byTime(events))
	var leases int
	var data []tsInt
	for _, e := range events {
		data = append(data, tsInt{e.time, leases})
		leases += e.lease
		data = append(data, tsInt{e.time, leases})
	}
	return graphTimeLine(data, tsLine{
		LineTitle:  "Active leases",
		OutputFile: "line.svg",
	})
}

type tsInt struct {
	time  time.Time
	value int
}

type tsLine struct {
	OutputFile string
	LineTitle  string
}

var (
	tsLineTmpl = template.Must(template.New("").Parse(`
set timefmt "%Y-%m-%d_%H:%M:%S"
set xdata time
set format x "%Y-%m-%d"
set xrange [ "2016-01-01":"2016-04-04" ]

set terminal svg size 800,300
set output "{{.OutputFile}}"
plot "-" using 1:2 w l title "{{.LineTitle}}"
`))
)

func graphTimeLine(ts []tsInt, data tsLine) error {
	if data.OutputFile == "" {
		return fmt.Errorf("must supply an output file name")
	}
	if data.LineTitle == "" {
		data.LineTitle = "data"
	}
	cmd := exec.Command("gnuplot")
	var def bytes.Buffer
	if err := tsLineTmpl.Execute(&def, &data); err != nil {
		return err
	}
	for _, s := range ts {
		fmt.Fprintf(&def, "%v %d\n", s.time.Format("2006-01-02_15:04:05"), s.value)
	}
	cmd.Stdin = &def
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func formatInt(i int64) string {
	var parts []string
	for i > 0 {
		parts = append([]string{fmt.Sprintf("%03d", i%1000)}, parts...)
		i /= 1000
	}
	ret := strings.TrimPrefix(strings.Join(parts, ","), "0")
	if ret == "" {
		ret = "0"
	}
	return ret
}

func formatFloat(in float64) string {
	i, f := math.Modf(in)
	fs := fmt.Sprint(f)[1:]
	return fmt.Sprintf("%s%s", formatInt(int64(i)), fs)
}

func formatDuration(t time.Duration) string {
	h, m, s := int(t.Hours()), int(t.Minutes()), int(t.Seconds())
	d := h / 24
	y := d / 365
	d %= 365
	h %= 24
	m %= 60
	s %= 60
	return fmt.Sprintf("%4dy %3dd %2dh %2dm %2ds", y, d, h, m, s)
}

func main() {
	flag.Parse()
	if flag.NArg() > 0 {
		log.Fatalf("Got extra args on cmdline: %q", flag.Args())
	}

	// Connect to database.
	var err error
	db, err = sql.Open("postgres", *dbConnect)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	if err := mkstats(); err != nil {
		log.Fatal(err)
	}
}
