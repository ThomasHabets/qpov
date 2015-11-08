package main

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
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
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/transport"

	"github.com/ThomasHabets/qpov/dist"
	pb "github.com/ThomasHabets/qpov/dist/qpov"
	"github.com/ThomasHabets/qpov/dist/rpclog"
)

var (
	defaultLeaseTime     = time.Hour
	db                   *sql.DB
	dbConnect            = flag.String("db", "", "")
	addr                 = flag.String("port", ":9999", "Addr to listen to.")
	certFile             = flag.String("cert_file", "", "The TLS cert file")
	keyFile              = flag.String("key_file", "", "The TLS key file")
	clientCAFile         = flag.String("client_ca_file", "", "The client CA file.")
	maxConcurrentStreams = flag.Int("max_concurrent_streams", 10000, "Max concurrent RPC streams.")
	rpclogDir            = flag.String("rpclog_dir", ".", "RPC log directory.")

	errNoCert = errors.New("no cert provided")
)

const (
	infoSuffix = ".json"
)

// getOwnerID from the RPC channel TLS client cert.
// Returns errNoCert if no cert is attached.
func getOwnerID(ctx context.Context) (int, error) {
	a, ok := credentials.FromContext(ctx)
	if !ok {
		return 0, errNoCert
	}
	at, ok := a.(credentials.TLSInfo)
	if !ok {
		return 0, fmt.Errorf("auth type is not TLSInfo")
	}
	if len(at.State.PeerCertificates) != 1 {
		return 0, errNoCert
	}
	cert := at.State.PeerCertificates[0]

	// If there is a cert then it was verified in the handshake as belonging to the client CA.
	// We just need to turn the cert CommonName into a userID.
	// TODO: Really the userID should be part of the cert instead of stored on the server side. Ugly hack for now.
	row := db.QueryRow(`SELECT users.user_id FROM users NATURAL JOIN certs WHERE certs.cn=$1`, cert.Subject.CommonName)
	var ownerID int
	if err := row.Scan(&ownerID); err == sql.ErrNoRows {
		return 0, fmt.Errorf("client cert not assigned to any user")
	} else if err != nil {
		return 0, fmt.Errorf("failed looking up cert: %v", err)
	}
	return ownerID, nil
}

type server struct {
	rpcLog *rpclog.Logger
}

func (s *server) Get(ctx context.Context, in *pb.GetRequest) (*pb.GetReply, error) {
	st := time.Now()
	requestID := uuid.New()
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
OR (FALSE AND leases.expires < NOW() AND leases.done = FALSE)
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

	var ownerID *int
	if o, err := getOwnerID(ctx); err == nil {
		ownerID = &o
	} else if err != errNoCert {
		log.Printf("Getting owner ID: %v", err)
	}

	lease := uuid.New()
	if _, err := tx.Exec(`INSERT INTO leases(lease_id, done, order_id, user_id, created, updated, expires)
VALUES($1, false, $2, $3, NOW(), NOW(), $4)`, lease, orderID, ownerID, time.Now().Add(defaultLeaseTime)); err != nil {
		log.Printf("Database error inserting lease: %v", err)
		return nil, fmt.Errorf("database error")
	}
	if err := tx.Commit(); err != nil {
		log.Printf("Database error committing: %v", err)
		return nil, fmt.Errorf("database error")
	}
	log.Printf("RPC(Get): Order: %q, Lease: %q", orderID, lease)
	ret := &pb.GetReply{
		LeaseId:         lease,
		OrderDefinition: def,
	}
	s.rpcLog.Log(ctx, requestID, "dscheduler.Get", st,
		"github.com/ThomasHabets/qpov/dist/qpov/GetRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/GetReply", ret)
	return ret, nil
}

func (s *server) Renew(ctx context.Context, in *pb.RenewRequest) (*pb.RenewReply, error) {
	st := time.Now()
	requestID := uuid.New()
	secs := in.ExtendSec
	if secs == 0 {
		secs = int32(defaultLeaseTime.Seconds())
	}
	n := time.Now().Add(time.Duration(int64(time.Second) * int64(secs)))
	_, err := db.Exec(`UPDATE leases SET updated=NOW(), expires=$1 WHERE lease_id=$2`, n, in.LeaseId)
	if err != nil {
		return nil, err
	}
	log.Printf("RPC(Renew): Lease: %q until %v", in.LeaseId, n)
	ret := &pb.RenewReply{
		NewTimeoutSec: int64(n.Unix()),
	}
	s.rpcLog.Log(ctx, requestID, "dscheduler.Renew", st,
		"github.com/ThomasHabets/qpov/dist/qpov/RenewRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/RenewReply", ret)
	return ret, nil
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
	st := time.Now()
	requestID := uuid.New()
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
	log.Printf("RPC(Done): Lease: %q", in.LeaseId)
	ret := &pb.DoneReply{}
	s.rpcLog.Log(ctx, requestID, "dscheduler.Done", st,
		"github.com/ThomasHabets/qpov/dist/qpov/DoneRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/DoneReply", ret)
	return ret, nil
}

func (s *server) Add(ctx context.Context, in *pb.AddRequest) (*pb.AddReply, error) {
	st := time.Now()
	requestID := uuid.New()
	var ownerID int
	{
		t, ok := transport.StreamFromContext(ctx)
		if !ok {
			return nil, fmt.Errorf("internal error: no stream context")
		}
		st := t.ServerTransport()
		log.Printf("Called from %v", st.RemoteAddr())
		a, ok := credentials.FromContext(ctx)
		if !ok {
			return nil, fmt.Errorf("can't add orders unauthenticated")
		}
		at, ok := a.(credentials.TLSInfo)
		if !ok {
			return nil, fmt.Errorf("internal error: auth type is not TLSInfo")
		}
		if len(at.State.PeerCertificates) != 1 {
			log.Printf("Add() attempted without cert from %v", st.RemoteAddr())
			return nil, fmt.Errorf("need a valid cert for operation 'Add'")
		}
		cert := at.State.PeerCertificates[0]

		// If there is a cert then it was verified in the handshake as belonging to the client CA.
		// We just need to turn the cert CommonName into a userID.
		// TODO: Really the userID should be part of the cert instead of stored on the server side. Ugly hack for now.
		row := db.QueryRow(`SELECT users.user_id, users.adding FROM users NATURAL JOIN certs WHERE certs.cn=$1`, cert.Subject.CommonName)
		var adding bool
		if err := row.Scan(&ownerID, &adding); err == sql.ErrNoRows {
			log.Printf("client cert %q not assigned to any user", cert.Subject.CommonName)
			return nil, fmt.Errorf("client cert not assigned to any user")
		} else if err != nil {
			log.Printf("Failed looking up cert %q: %v", cert.Subject.CommonName, err)
			return nil, fmt.Errorf("internal error: failed looking up cert")
		}
		if !adding {
			log.Printf("User not allowed to add orders")
			return nil, fmt.Errorf("user not allowed to add orders")
		}
	}

	id := uuid.New()
	_, err := db.Exec(`INSERT INTO orders(order_id, owner, definition) VALUES($1,$2,$3)`, id, ownerID, in.OrderDefinition)
	if err != nil {
		return nil, err
	}
	log.Printf("RPC(Add): Order: %q", id)

	ret := &pb.AddReply{OrderId: id}
	s.rpcLog.Log(ctx, requestID, "dscheduler.Add", st,
		"github.com/ThomasHabets/qpov/dist/qpov/AddRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/AddReply", ret)
	return ret, nil
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
	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(uint32(*maxConcurrentStreams)),
	}
	if *certFile != "" {
		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			log.Fatalf("Failed to load certs: %v", err)
		}
		b, err := ioutil.ReadFile(*clientCAFile)
		if err != nil {
			log.Fatalf("reading %q: %v", *clientCAFile, err)
		}
		cp := x509.NewCertPool()
		if ok := cp.AppendCertsFromPEM(b); !ok {
			log.Fatalf("failed to add client CAs")
		}
		t := &tls.Config{
			ClientAuth:   tls.VerifyClientCertIfGiven,
			ClientCAs:    cp,
			Certificates: []tls.Certificate{cert},
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(t)))
	}
	s := grpc.NewServer(opts...)
	now := time.Now()
	fin, err := os.Create(path.Join(*rpclogDir, fmt.Sprintf("rpclog.%d.in.gob", now.Unix())))
	if err != nil {
		log.Fatalf("Opening rpclog: %v", err)
	}
	fout, err := os.Create(path.Join(*rpclogDir, fmt.Sprintf("rpclog.%d.out.gob", now.Unix())))
	if err != nil {
		log.Fatalf("Opening rpclog: %v", err)
	}
	l := rpclog.New(fin, fout)
	pb.RegisterSchedulerServer(s, &server{rpcLog: l})
	log.Printf("Running...")
	log.Fatal(s.Serve(lis))
}
