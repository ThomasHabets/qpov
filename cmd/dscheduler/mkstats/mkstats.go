package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"

	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

var (
	db *sql.DB
	// TODO: Ask the scheduler for the leases instead of getting them from the DB directly.
	dbConnect = flag.String("db", "", "Database connect string.")
)

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
	type event struct {
		time    time.Time
		lease   int
		cpuTime int64
	}
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
				time:  time.Unix(stats.StartMs*1000000, 0),
				lease: 1,
			},
			event{
				time:  time.Unix(stats.EndMs*1000000, 0),
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
	return nil
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
