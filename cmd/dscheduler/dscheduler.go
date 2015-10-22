package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"regexp"
	"sync"
	"time"

	"github.com/ThomasHabets/go-uuid/uuid"
	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/s3"
	_ "github.com/lib/pq"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/ThomasHabets/qpov/dist"
	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

var (
	dbConnect        = flag.String("db", "", "")
	defaultLeaseTime = time.Hour
	db               *sql.DB
	addr             = flag.String("port", ":9999", "Addr to listen to.")
)

const (
	infoSuffix = ".json"
)

type server struct{}

func (s *server) Get(ctx context.Context, in *pb.GetRequest) (*pb.GetReply, error) {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Database error: %v", err)
		return nil, fmt.Errorf("database error")
	}

	defer tx.Rollback()
	row := tx.QueryRow(`
SELECT orders.order_id, orders.definition
FROM orders
LEFT OUTER JOIN leases ON orders.order_id=leases.order_id
WHERE leases.lease_id IS NULL
OR (leases.expires < NOW()
    AND leases.done = FALSE)
LIMIT 1`)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("nothing to do")
	} else if err != nil {
		log.Printf("Database error QueryRow: %v", err)
		return nil, fmt.Errorf("database error")
	}

	var orderID, def string
	if err := row.Scan(&orderID, &def); err == sql.ErrNoRows {
		return nil, fmt.Errorf("nothing to do")
	} else if err != nil {
		log.Printf("Database error scanning: %v", err)
		return nil, fmt.Errorf("database error")
	}

	lease := uuid.New()
	if _, err := tx.Exec(`INSERT INTO leases(lease_id, done, order_id, user_id, created, updated, expires)
VALUES($1, false, $2, $3, NOW(), NOW(), $4)`, lease, orderID, 1, time.Now().Add(defaultLeaseTime)); err != nil {
		log.Printf("Database error inserting lease: %v", err)
		return nil, fmt.Errorf("database error")
	}
	if err := tx.Commit(); err != nil {
		log.Printf("Database error committing: %v", err)
		return nil, fmt.Errorf("database error")
	}
	return &pb.GetReply{
		LeaseId:         lease,
		OrderDefinition: def,
	}, nil
}

func (s *server) Renew(ctx context.Context, in *pb.RenewRequest) (*pb.RenewReply, error) {
	_, err := db.Exec(`UPDATE leases SET updated=NOW(), expires=$1 WHERE lease_id=$2`, time.Now().Add(defaultLeaseTime), in.LeaseId)
	if err != nil {
		return nil, err
	}
	return &pb.RenewReply{}, nil
}

func getOrderDestByLeaseID(id string) (string, string, error) {
	row := db.QueryRow(`SELECT orders.definition FROM orders NATURAL JOIN leases WHERE lease_id=$1`, id)
	var def string
	if err := row.Scan(&def); err != nil {
		return "", "", err
	}
	var order dist.Order
	if err := json.Unmarshal([]byte(def), &order); err != nil {
		return "", "", err
	}
	return order.Destination, order.File, nil
}

func getAuth() aws.Auth {
	return aws.Auth{
		AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}
}

func (s *server) Done(ctx context.Context, in *pb.DoneRequest) (*pb.DoneReply, error) {
	// First give us time to receive the data.
	_, err := s.Renew(ctx, &pb.RenewRequest{
		LeaseId: in.LeaseId,
	})
	if err != nil {
		return nil, err
	}

	// Fetch the order. Needed for the destination.
	destination, file, err := getOrderDestByLeaseID(in.LeaseId)
	if err != nil {
		log.Printf("Can't find order with lease %q: %v", in.LeaseId, err)
		return nil, fmt.Errorf("unknown lease %q", in.LeaseId)
	}

	sthree := s3.New(getAuth(), aws.USEast, nil)
	bucket, destDir, _, _ := dist.S3Parse(destination)
	b := sthree.Bucket(bucket)

	var wg sync.WaitGroup

	re := regexp.MustCompile(`\.pov$`)
	image := re.ReplaceAllString(file, ".png")

	files := []struct {
		ct   string
		fn   string
		data []byte
	}{
		{"image/png", image, in.Image},
		{"text/plain", file + ".stdout", in.Stdout},
		{"text/plain", file + ".stderr", in.Stderr},
		{"text/plain", file + infoSuffix, []byte(in.JsonMetadata)},
	}
	errCh := make(chan error, len(files))
	wg.Add(len(files))
	acl := s3.ACL("")
	for _, e := range files {
		e := e
		go func() {
			defer wg.Done()
			if err := b.Put(path.Join(destDir, e.fn), e.data, e.ct, acl, s3.Options{}); err != nil {
				log.Printf("S3 upload of %q error: %v", e.fn, err)
				errCh <- fmt.Errorf("S3 upload error")
			}
		}()
	}
	go func() {
		wg.Wait()
		close(errCh)
	}()
	if err := <-errCh; err != nil {
		return nil, err
	}
	// Mark as completed.
	if _, err := db.Exec(`UPDATE leases SET done=TRUE WHERE lease_id=$1`, in.LeaseId); err != nil {
		return nil, err
	}
	return &pb.DoneReply{}, nil
}

func (s *server) Add(ctx context.Context, in *pb.AddRequest) (*pb.AddReply, error) {
	id := uuid.New()
	_, err := db.Exec(`INSERT INTO orders(order_id, owner, definition) VALUES($1,$2,$3)`, id, 1, in.OrderDefinition)
	if err != nil {
		return nil, err
	}
	return &pb.AddReply{
		OrderId: id,
	}, nil
}

func main() {
	flag.Parse()
	var err error
	db, err = sql.Open("postgres", *dbConnect)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterSchedulerServer(s, &server{})
	log.Printf("Running...")
	log.Fatal(s.Serve(lis))
}
