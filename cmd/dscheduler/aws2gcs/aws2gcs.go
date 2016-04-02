// aws2gcs is a one-time script for moving old results from AWS S3 to GCS.
// AWS has json and plaintext that is then also converted to a binary protobuf.
// Yes, this tool has a bad name, and "s32gcs" or "aws2gcp" would be better.
package main

import (
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/s3"
	"github.com/golang/protobuf/proto"
	_ "github.com/lib/pq"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/cloud"
	"google.golang.org/cloud/storage"

	"github.com/ThomasHabets/qpov/dist"
	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

var (
	//awsBucket = flag.String("aws_bucket", "", "Source AWS bucket.")
	gcsBucket = flag.String("gcs_bucket", "", "Destination GCS bucket.")

	gcsCredentials = flag.String("gcs_credentials", "", "JSON file containing credentials.")
	dbConnect      = flag.String("db", "", "")

	// Connection handles.
	db     *sql.DB
	gcs    *storage.Client
	sthree *s3.S3
)

func getAWSAuth() aws.Auth {
	return aws.Auth{
		AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}
}

func readS3File(b *s3.Bucket, f string) ([]byte, error) {
	r, err := b.GetReader(f)
	if err != nil {
		return nil, err
	}
	ret, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func copyFile(ctx context.Context, srcbucket, src, dst string) {
	base := path.Base(src)
	//log.Printf("Would copy file %q %q to %q/%q", srcbucket, src, dst, base)

	// Copy the PNG.
	if false {
		o := gcs.Bucket(*gcsBucket).Object(path.Join(dst, base+".png"))
		if _, err := o.Attrs(ctx); err == nil {
			// Already copied.
			return
		} else if err == storage.ErrObjectNotExist {
			// Not copied yet.
		} else {
			log.Fatal(err)
		}

		r, err := sthree.Bucket(srcbucket).GetReader(src + ".png")
		if err != nil {
			log.Printf("bucket.GetReader(%q): %v", src, err)
			return
		}
		w := o.NewWriter(ctx)
		log.Printf("Copying %q %q to %q/%q", srcbucket, src, dst, base)
		if err := func() error {
			if _, err := io.Copy(w, r); err != nil {
				return err
			}
			return w.Close()
		}(); err != nil {
			w.CloseWithError(err)
			log.Fatal(err)
		}
	}

	if true {
		o := gcs.Bucket(*gcsBucket).Object(path.Join(dst, base+".meta.pb.gz"))
		if _, err := o.Attrs(ctx); err == nil {
			// Already copied.
			return
		} else if err == storage.ErrObjectNotExist {
			// Not copied yet.
		} else {
			log.Fatal(err)
		}
		w := o.NewWriter(ctx)
		wz := gzip.NewWriter(w)

		log.Printf("Metadata %q %q to %q/%q", srcbucket, src, dst, base)

		// Read JSON.
		j, err := readS3File(sthree.Bucket(srcbucket), src+".pov.json")
		if err != nil {
			log.Printf("Reading json: %v", err)
			return
		}

		// Read stdout.
		stdout, err := readS3File(sthree.Bucket(srcbucket), src+".pov.stdout")
		if err != nil {
			log.Fatalf("Reading stdout: %v", err)
		}

		// Read stderr.
		stderr, err := readS3File(sthree.Bucket(srcbucket), src+".pov.stderr")
		if err != nil {
			log.Fatalf("Reading stderr: %v", err)
		}

		var meta pb.RenderingMetadata
		if err := json.Unmarshal(j, &meta); err != nil {
			t, err := dist.ParseLegacyJSON(j)
			if err != nil {
				log.Printf("Parsing legacy JSON: %v", err)
				return
			}
			meta = *t
		}
		if len(meta.Stdout) == 0 {
			meta.Stdout = stdout
		}
		if len(meta.Stderr) == 0 {
			meta.Stderr = stderr
		}
		metab, err := proto.Marshal(&meta)
		if err != nil {
			log.Fatal(err)
		}
		if err := func() error {
			if n, err := wz.Write(metab); err != nil {
				return err
			} else if n != len(metab) {
				return fmt.Errorf("short write %d < %d", n, len(metab))
			}
			if err := wz.Close(); err != nil {
				return err
			}
			return w.Close()
		}(); err != nil {
			w.CloseWithError(err)
			log.Fatal(err)
		}
	}
}

var mu sync.Mutex

func copyLease(ctx context.Context, leaseID, orderID string) {
	var batch sql.NullString
	var definition string
	func() {
		mu.Lock()
		defer mu.Unlock()
		err := db.QueryRow(`SELECT batch_id, definition FROM orders WHERE order_id=$1`, orderID).Scan(&batch, &definition)
		if err != nil {
			log.Fatal("Db query: %v", err)
		}
	}()
	dstDir := ""
	if batch.Valid {
		dstDir = path.Join(dstDir, "batch", batch.String, leaseID)
	} else {
		dstDir = path.Join(dstDir, "single", leaseID)
	}

	var order dist.Order
	if err := json.Unmarshal([]byte(definition), &order); err != nil {
		log.Fatal(err)
	}

	bucket, srcDir, _, err := dist.S3Parse(order.Destination)
	if err != nil {
		log.Fatal(err)
	}
	copyFile(ctx, bucket, path.Join(srcDir, leaseID, strings.TrimSuffix(order.File, path.Ext(order.File))), dstDir)
}

func copyAll(ctx context.Context) {
	rows, err := db.Query(`SELECT lease_id, order_id FROM leases WHERE done=TRUE`)
	if err != nil {
		log.Fatal(err)
	}

	type e struct {
		leaseID, orderID string
	}
	all := []e{}
	for rows.Next() {
		var leaseID, orderID string
		if err := rows.Scan(&leaseID, &orderID); err != nil {
			log.Fatal(err)
		}
		all = append(all, e{
			leaseID: leaseID,
			orderID: orderID,
		})
	}
	rows.Close()
	var wg sync.WaitGroup
	for _, t := range all {
		wg.Add(1)
		t := t
		go func() {
			defer wg.Done()
			copyLease(ctx, t.leaseID, t.orderID)
		}()
	}
	wg.Wait()
}

func main() {
	flag.Parse()
	if flag.NArg() > 0 {
		log.Fatalf("Got extra args on cmdline: %q", flag.Args())
	}

	ctx := context.Background()

	// Connect to GCS.
	{
		jsonKey, err := ioutil.ReadFile(*gcsCredentials)
		if err != nil {
			log.Fatal(err)
		}
		conf, err := google.JWTConfigFromJSON(
			jsonKey,
			storage.ScopeFullControl,
		)
		if err != nil {
			log.Fatal(err)
		}
		gcs, err = storage.NewClient(ctx, cloud.WithTokenSource(conf.TokenSource(ctx)))
		if err != nil {
			log.Fatal(err)
		}
	}

	// Connect to AWS.
	sthree = s3.New(getAWSAuth(), aws.USEast, nil)

	// Connect to database.
	var err error
	db, err = sql.Open("postgres", *dbConnect)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("db ping: %v", err)
	}
	log.Printf("Running aws2gcs")
	copyAll(ctx)
	log.Printf("Done")
}
