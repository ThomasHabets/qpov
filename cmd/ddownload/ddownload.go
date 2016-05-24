// Download all rendered files into a zipfile.
// TODO: this should connect to dscheduler instead of db & cloud, right?
package main

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/cloud"
	"google.golang.org/cloud/storage"

	"github.com/ThomasHabets/qpov/dist"
	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

var (
	out                 = flag.String("out", "", "Output zipfile.")
	dbConnect           = flag.String("db", "", "")
	cloudCredentials    = flag.String("cloud_credentials", "", "Path to JSON file containing credentials.")
	downloadBucketNames = flag.String("cloud_download_buckets", "", "Google cloud storage bucket name to read from.")
	batchID             = flag.String("batch", "", "Batch to download.")

	db                 dist.DBWrap
	googleCloudStorage *storage.Client
)

func getLeases() ([]string, error) {
	rows, err := db.Query(`
SELECT
  orders.definition,
  CAST(MIN(CAST(orders.order_id AS TEXT)) AS UUID)
FROM leases
JOIN  orders ON leases.order_id=orders.order_id
WHERE orders.batch_id=$1
AND   leases.done=TRUE
GROUP BY orders.order_id,orders.definition
`, *batchID)
	if err != nil {
		return nil, err
	}
	t := make(map[string]string)
	var files []string
	for rows.Next() {
		var def string
		var leaseID string
		if err := rows.Scan(&def, &leaseID); err != nil {
			return nil, err
		}
		var p pb.Order
		if err := json.Unmarshal([]byte(def), &p); err != nil {
			return nil, err
		}
		t[p.File] = leaseID
		files = append(files, p.File)
	}
	if rows.Err() != nil {
		return nil, err
	}
	sort.Strings(files)
	var ps []string
	for _, f := range files {
		ps = append(ps, path.Join(t[f], f))
	}
	return ps, nil
}

func tryDownload(f, bucketName string) ([]byte, error) {
	// TODO: actually download them.
	return []byte("hello"), nil
}

func ddownload(o *zip.Writer) error {
	leases, err := getLeases()
	if err != nil {
		return err
	}
	log.Printf("Frames: %d", len(leases))
	if true {
	leaseLoop:
		for _, f := range leases {
			log.Printf("  Downloading %s...", f)
			for {
				for _, bucketName := range strings.Split(*downloadBucketNames, ",") {
					buf, err := tryDownload(f, bucketName)
					if err != nil {
						log.Printf("Failed to download %q from %q: %v", f, bucketName, err)
						continue
					}
					w, err := o.Create(path.Base(f))
					if err != nil {
						return err
					}
					if _, err := w.Write(buf); err != nil {
						return err
					}
					continue leaseLoop
				}
				log.Printf("Failed to download %q from all buckets, sleeping a bit")
				time.Sleep(time.Minute)
				log.Printf("Trying again")
			}
			// Can't be reached at the moment.
			return fmt.Errorf("failed to download %q", f)
		}
	}
	return nil
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)
	log.Printf("Running ddownload")
	flag.Parse()
	if flag.NArg() > 0 {
		log.Fatalf("Got extra args on cmdline: %q", flag.Args())
	}

	fo, err := os.Create(*out)
	if err != nil {
		log.Fatalf("Failed to create %q: %v", *out, err)
	}
	defer func() {
		if err := fo.Close(); err != nil {
			log.Fatalf("Failed to close %q: %v", *out, err)
		}
	}()
	fz := zip.NewWriter(fo)
	defer func() {
		if err := fz.Close(); err != nil {
			log.Fatalf("Failed to close zip writer for %q: %v", *out, err)
		}
	}()

	// Connect to database.
	{
		t, err := sql.Open("postgres", *dbConnect)
		if err != nil {
			log.Fatal(err)
		}
		if err := t.Ping(); err != nil {
			log.Fatalf("db ping: %v", err)
		}
		db = dist.NewDBWrap(t, log.New(os.Stderr, "", log.LstdFlags))
	}

	ctx := context.Background()

	// Connect to GCS.
	{
		jsonKey, err := ioutil.ReadFile(*cloudCredentials)
		if err != nil {
			log.Fatalf("Reading %q: %v", *cloudCredentials, err)
		}
		conf, err := google.JWTConfigFromJSON(
			jsonKey,
			storage.ScopeReadOnly,
		)
		if err != nil {
			log.Fatal(err)
		}
		googleCloudStorage, err = storage.NewClient(ctx, cloud.WithTokenSource(conf.TokenSource(ctx)))
		if err != nil {
			log.Fatal(err)
		}
	}

	if err := ddownload(fz); err != nil {
		log.Fatal(err)
	}
}
