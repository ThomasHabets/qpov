// Read metadata from file into SQL.
// This should be a one-time op since dscheduler now writes this at Done-time.
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"

	_ "github.com/lib/pq"

	"github.com/ThomasHabets/qpov/dist"
)

var (
	dbConnect = flag.String("db", "", "Database connect string.")
	leaseID   = flag.String("lease", "", "Lease ID.")
	jfile     = flag.String("in", "", "Json file to import.")
)

func main() {
	flag.Parse()
	if flag.NArg() != 0 {
		log.Fatalf("Extra args on cmdline: %q", flag.Args())
	}

	db, err := sql.Open("postgres", *dbConnect)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	buf, err := ioutil.ReadFile(*jfile)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	st, err := dist.ParseLegacyJSON(buf)
	if err != nil {
		log.Fatalf("Failed to convert to proto: %v", err)
	}

	b, err := json.Marshal(st)
	if err != nil {
		log.Fatalf("Failed to marshal proto: %v", err)
	}
	if _, err := db.Exec(`UPDATE leases SET metadata=$2 WHERE lease_id=$1`, *leaseID, string(b)); err != nil {
		log.Fatal(err)
	}
}
