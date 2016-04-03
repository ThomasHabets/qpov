package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
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
		cpuUS += stats.Rusage.Utime + stats.Rusage.Stime
		realMS += stats.EndMs - stats.StartMs
	}
	if rows.Err() != nil {
		return err
	}
	fmt.Printf("CPU time:     %v\n", formatDuration(time.Duration(cpuUS*1000)))
	fmt.Printf("Machine time: %v\n", formatDuration(time.Duration(realMS*1000000)))
	return nil
}

func formatDuration(t time.Duration) string {
	h, m, s := int(t.Hours()), int(t.Minutes()), int(t.Seconds())
	d := h / 24
	y := d / 365
	d %= 365
	h %= 24
	m %= 60
	s %= 60
	return fmt.Sprintf("%4dy %2dd %2dh %2dm %2ds", y, d, h, m, s)
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
