// Download all rendered files into a zipfile.
// TODO: this should connect to dscheduler instead of db & cloud, right?
package main

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	storage "cloud.google.com/go/storage"
	_ "github.com/lib/pq"
	"golang.org/x/net/context"
	cloudopt "google.golang.org/api/option"

	"github.com/ThomasHabets/qpov/dist"
	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

var (
	out                 = flag.String("out", "", "Output zipfile.")
	dbConnect           = flag.String("db", "", "")
	cloudCredentials    = flag.String("cloud_credentials", "", "Path to JSON file containing credentials.")
	downloadBucketNames = flag.String("cloud_download_buckets", "", "Google cloud storage bucket name to read from.")
	batchID             = flag.String("batch", "", "Batch to download.")
	dryRun              = flag.Bool("dry_run", false, "Put fake data in zip file instead of downloading from cloud.")

	db                 dist.DBWrap
	googleCloudStorage *storage.Client
)

func replaceExt(fn, n string) string {
	return fn[0:len(fn)-len(path.Ext(fn))] + n
}

func getLeases(ctx context.Context) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
  orders.definition,
  CAST(MIN(CAST(leases.lease_id AS TEXT)) AS UUID)
FROM leases
JOIN  orders ON leases.order_id=orders.order_id
WHERE orders.batch_id=$1
AND   leases.done=TRUE
GROUP BY orders.definition
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
		fn := replaceExt(p.File, ".png")
		t[fn] = leaseID
		files = append(files, fn)
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

func tryDownload(ctx context.Context, f, bucketName string) ([]byte, error) {
	if *dryRun {
		return []byte("hello"), nil
	}
	r, err := googleCloudStorage.Bucket(bucketName).Object(path.Join("batch", *batchID, f)).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func ddownload(ctx context.Context, o *zip.Writer) error {
	leases, err := getLeases(ctx)
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
					buf, err := tryDownload(ctx, f, bucketName)
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
				log.Printf("Failed to download %q from all buckets, sleeping a bit", f)
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

	if *downloadBucketNames == "" {
		log.Fatalf("-cloud_download_buckets is mandatory")
	}
	if *cloudCredentials == "" {
		log.Fatalf("-cloud_credentials is mandatory")
	}
	if *out == "" {
		log.Fatalf("-out is mandatory")
	}
	if *batchID == "" {
		log.Fatalf("-batch is mandatory")
	}

	fo, err := os.Create(*out)
	if err != nil {
		log.Fatalf("Failed to create -out %q: %v", *out, err)
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
		if err := t.PingContext(); err != nil {
			log.Fatalf("db ping: %v", err)
		}
		db = dist.NewDBWrap(t, log.New(os.Stderr, "", log.LstdFlags))
	}

	ctx := context.Background()

	// Connect to GCS.
	{ /*
			jsonKey, err := ioutil.ReadFile(*cloudCredentials)
			if err != nil {
				log.Fatalf("Reading -cloud_credentials %q: %v", *cloudCredentials, err)
			}
			conf, err := google.JWTConfigFromJSON(
				jsonKey,
				storage.ScopeReadOnly,
			)
			if err != nil {
				log.Fatal(err)
			}
			conf = conf
		*/
		googleCloudStorage, err = storage.NewClient(ctx, cloudopt.WithServiceAccountFile(*cloudCredentials)) //, cloud.WithTokenSource(conf.TokenSource(ctx)))
		if err != nil {
			log.Fatal(err)
		}
		defer googleCloudStorage.Close()
	}

	if err := ddownload(ctx, fz); err != nil {
		log.Fatal(err)
	}
}
