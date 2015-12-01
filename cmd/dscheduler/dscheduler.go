package main

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ThomasHabets/go-uuid/uuid"
	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/s3"
	_ "github.com/lib/pq"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/transport"

	"github.com/ThomasHabets/qpov/dist"
	pb "github.com/ThomasHabets/qpov/dist/qpov"
	"github.com/ThomasHabets/qpov/dist/rpclog"
)

var (
	defaultLeaseRenewTime = time.Hour
	maxLeaseRenewTime     = 48 * time.Hour

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

// Log error and return gRPC error safe for returing to users.
func dbError(doing string, err error) error {
	return internalError("database error", "%s: %v", doing, err)
}

func internalError(public string, f string, a ...interface{}) error {
	log.Printf(f, a...)
	return grpc.Errorf(codes.Internal, public)
}

// If error is not gRPC error, log and return "clean" error.
// If it is a gRPC error, just use that, it's already clean.
func cleanError(err error, code codes.Code, public string, a ...interface{}) error {
	if grpc.Code(err) != codes.Unknown {
		return err
	}
	log.Printf("%v: %v", fmt.Sprintf(public, a...), err)
	return grpc.Errorf(code, public, a...)
}

func getOrderByID(id string) (*pb.Order, error) {
	row := db.QueryRow(`SELECT definition FROM orders WHERE order_id=$1`, id)
	var def string
	if err := row.Scan(&def); err != nil {
		return nil, err
	}
	var order dist.Order
	if err := json.Unmarshal([]byte(def), &order); err != nil {
		return nil, err
	}

	return &pb.Order{
		Package: order.Package,
		Dir:     order.Dir,
		File:    order.File,
		Args:    order.Args,
	}, nil
}

func getOwnerIDByCN(cn string) (int, error) {
	row := db.QueryRow(`SELECT users.user_id FROM users JOIN certs ON users.user_id=certs.user_id WHERE certs.cn=$1`, cn)
	var ownerID int
	if err := row.Scan(&ownerID); err == sql.ErrNoRows {
		return 0, fmt.Errorf("client cert not assigned to any user")
	} else if err != nil {
		return 0, fmt.Errorf("failed looking up cert: %v", err)
	}
	return ownerID, nil
}

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
	ownerID, err := getOwnerIDByCN(cert.Subject.CommonName)
	if err == nil {
		return ownerID, nil
	}
	aa := strings.Split(cert.Subject.CommonName, ":")
	if len(aa) == 2 {
		if ownerID, err := getOwnerIDByCN(aa[0]); err == nil {
			return ownerID, nil
		}
	}
	return 0, err
}

type server struct {
	rpcLog *rpclog.Logger
}

func (s *server) Get(ctx context.Context, in *pb.GetRequest) (*pb.GetReply, error) {
	st := time.Now()
	requestID := uuid.New()

	tx, err := db.Begin()
	if err != nil {
		return nil, dbError("begin transaction", err)
	}
	defer tx.Rollback()

	// TODO: Hand out expired and failed leases too.
	row := tx.QueryRow(`
SELECT orders.order_id, orders.definition
FROM orders
LEFT OUTER JOIN leases ON orders.order_id=leases.order_id
WHERE leases.lease_id IS NULL
OR (FALSE AND leases.expires < NOW() AND leases.done = FALSE)
ORDER BY RANDOM()
LIMIT 1`)

	var orderID, def string
	if err := row.Scan(&orderID, &def); err == sql.ErrNoRows {
		return nil, grpc.Errorf(codes.NotFound, "nothing to do")
	} else if err != nil {
		return nil, dbError("Scanning order", err)
	}

	var ownerID *int
	if o, err := getOwnerID(ctx); err == nil {
		ownerID = &o
	} else if err != errNoCert {
		log.Printf("Getting owner ID: %v", err)
	}

	lease := uuid.New()
	if _, err := tx.Exec(`INSERT INTO leases(lease_id, done, order_id, user_id, created, updated, expires)
VALUES($1, false, $2, $3, NOW(), NOW(), $4)`, lease, orderID, ownerID, time.Now().Add(defaultLeaseRenewTime)); err != nil {
		return nil, dbError("Inserting lease", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, dbError("Committing transaction", err)
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

// renew is the backend for Renew().
func (s *server) renew(ctx context.Context, lease string, secs int32) (time.Time, error) {
	if secs <= 0 {
		secs = int32(defaultLeaseRenewTime.Seconds())
	}
	if secs > int32(maxLeaseRenewTime.Seconds()) {
		secs = int32(maxLeaseRenewTime.Seconds())
	}
	n := time.Now().Add(time.Duration(int64(time.Second) * int64(secs)))
	if _, err := db.Exec(`UPDATE leases SET updated=NOW(), expires=$1 WHERE lease_id=$2 AND done=FALSE AND failed=FALSE`, n, lease); err != nil {
		return time.Now(), dbError("Updating lease", err)
	}
	return n, nil
}

func (s *server) Renew(ctx context.Context, in *pb.RenewRequest) (*pb.RenewReply, error) {
	st := time.Now()
	requestID := uuid.New()
	n, err := s.renew(ctx, in.LeaseId, in.ExtendSec)
	if err != nil {
		return nil, cleanError(err, codes.Internal, "You got the error message for the error message, you win.")
	}
	log.Printf("RPC(Renew): Lease: %q until %v", in.LeaseId, n)
	ret := &pb.RenewReply{
		NewTimeoutSec: n.UnixNano() / 1000000000,
	}
	s.rpcLog.Log(ctx, requestID, "dscheduler.Renew", st,
		"github.com/ThomasHabets/qpov/dist/qpov/RenewRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/RenewReply", ret)
	return ret, nil
}

func getOrderDestByLeaseID(id string) (string, string, error) {
	row := db.QueryRow(`SELECT orders.definition FROM orders JOIN leases ON orders.order_id=leases.order_id WHERE lease_id=$1`, id)
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

func (s *server) Failed(ctx context.Context, in *pb.FailedRequest) (*pb.FailedReply, error) {
	st := time.Now()
	requestID := uuid.New()

	if _, err := db.Exec(`
UPDATE leases
SET
  failed=TRUE,
  updated=NOW()
WHERE done=FALSE
AND   failed=FALSE
AND   lease_id=$1`, in.LeaseId); err != nil {
		return nil, dbError("Marking failed", err)
	}

	log.Printf("RPC(Failed): Lease: %q", in.LeaseId)
	ret := &pb.FailedReply{}
	s.rpcLog.Log(ctx, requestID, "dscheduler.Failed", st,
		"github.com/ThomasHabets/qpov/dist/qpov/FailedRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/FailedReply", ret)
	return ret, nil
}

func leaseDone(id string) (bool, bool, error) {
	row := db.QueryRow(`SELECT done, failed FROM leases WHERE lease_id=$1`, id)
	var done, failed bool
	if err := row.Scan(&done, &failed); err != nil {
		return false, false, err
	}
	return done, failed, nil
}

func (s *server) Done(ctx context.Context, in *pb.DoneRequest) (*pb.DoneReply, error) {
	st := time.Now()
	requestID := uuid.New()
	// First give us time to receive the data.
	if _, err := s.renew(ctx, in.LeaseId, -1); err != nil {
		log.Printf("RPC(Done): Failed to renew before Done'ing: %v", err)
	}
	isDone, isFailed, err := leaseDone(in.LeaseId)
	if err != nil {
		return nil, dbError(fmt.Sprintf("Failed looking up lease %q", in.LeaseId), err)
	}
	if isDone {
		return nil, grpc.Errorf(codes.AlreadyExists, "lease already done: %q", in.LeaseId)
	}
	if isFailed {
		return nil, grpc.Errorf(codes.AlreadyExists, "lease already failed: %q", in.LeaseId)
	}

	// Fetch the order. Needed for the destination.
	destination, file, err := getOrderDestByLeaseID(in.LeaseId)
	if err != nil {
		log.Printf("RPC(Done): Can't find order with lease %q: %v", in.LeaseId, err)
		return nil, grpc.Errorf(codes.NotFound, "unknown lease %q", in.LeaseId)
	}

	sthree := s3.New(getAuth(), aws.USEast, nil)
	bucket, destDir, _, _ := dist.S3Parse(destination)
	b := sthree.Bucket(bucket)
	destDir = path.Join(destDir, in.LeaseId)
	var wg sync.WaitGroup

	re := regexp.MustCompile(`\.pov$`)
	image := re.ReplaceAllString(file, ".png")

	// Create metadata to store in DB from oldJSON.
	var newStats string
	{
		stats, err := dist.ParseLegacyJSON([]byte(in.JsonMetadata))
		if err != nil {
			log.Printf("Warning: bad JSON: %v", err)
		}
		b, err := json.Marshal(stats)
		if err != nil {
			log.Printf("Warning: failed to re-encode to newJSON: %v", err)
		}
		newStats = string(b)
	}

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
		return nil, internalError("storing results", "Uploading to S3: %v", err)
	}
	// Mark as completed.
	if _, err := db.Exec(`UPDATE leases SET done=TRUE,updated=NOW(),metadata=$2 WHERE lease_id=$1`, in.LeaseId, newStats); err != nil {
		return nil, dbError("Marking done", err)
	}
	log.Printf("RPC(Done): Lease: %q", in.LeaseId)
	ret := &pb.DoneReply{}
	s.rpcLog.Log(ctx, requestID, "dscheduler.Done", st,
		"github.com/ThomasHabets/qpov/dist/qpov/DoneRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/DoneReply", ret)
	return ret, nil
}

func (s *server) Result(in *pb.ResultRequest, stream pb.Scheduler_ResultServer) error {
	st := time.Now()
	requestID := uuid.New()
	ctx := stream.Context()
	if err := blockRestrictedAPI(ctx); err != nil {
		return err
	}

	// Send initial metadata.
	ret := pb.ResultReply{
		ContentType: "image/png",
	}
	{
		if err := stream.Send(&ret); err != nil {
			return internalError("failed to stream results", "failed to stream results: %v", err)
		}
	}

	// Send data, if requested.
	if in.Data {
		destination, file, err := getOrderDestByLeaseID(in.LeaseId)
		if err != nil {
			log.Printf("Can't find order with lease %q: %v", in.LeaseId, err)
			return grpc.Errorf(codes.NotFound, "unknown lease %q", in.LeaseId)
		}
		sthree := s3.New(getAuth(), aws.USEast, nil)
		bucket, destDir, _, _ := dist.S3Parse(destination)
		b := sthree.Bucket(bucket)
		destDir2 := path.Join(destDir, in.LeaseId)
		re := regexp.MustCompile(`\.pov$`)
		image := re.ReplaceAllString(file, ".png")
		fn := path.Join(destDir, image)
		fn2 := path.Join(destDir2, image)
		r, err := b.GetReader(fn)
		if err != nil {
			// Look in per-lease dir too.
			r, err = b.GetReader(fn2)
			if err != nil {
				log.Printf("File %q not found on S3: %v", fn, err)
				return grpc.Errorf(codes.NotFound, "file not found")
			}
		}
		defer r.Close()
		for {
			buf := make([]byte, 1024, 1024)
			n, err := r.Read(buf)
			if err == io.EOF {
				break
			}
			if err != nil {
				return internalError("failed to stream result", "failed to stream result %q: %v", image, err)
			}
			if err := stream.Send(&pb.ResultReply{Data: buf[0:n]}); err != nil {
				return internalError("failed to stream result", "failed to stream result: %v", err)
			}
		}
	}

	log.Printf("RPC(Result)")
	s.rpcLog.Log(ctx, requestID, "dscheduler.Result", st,
		"github.com/ThomasHabets/qpov/dist/qpov/ResultRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/ResultReply", &ret)
	return nil
}

func (s *server) Orders(in *pb.OrdersRequest, stream pb.Scheduler_OrdersServer) error {
	st := time.Now()
	requestID := uuid.New()
	ctx := stream.Context()
	if err := blockRestrictedAPI(ctx); err != nil {
		return err
	}
	having := []string{"TRUE=FALSE"}
	if in.Active {
		having = append(having, "COUNT(activeleases.lease_id)>0")
	}
	if in.Done {
		having = append(having, "COUNT(doneleases.lease_id)>0")
	}
	if in.Unstarted {
		having = append(having, "(COUNT(activeleases.lease_id)=0 AND COUNT(doneleases.lease_id)=0)")
	}
	rows, err := db.Query(fmt.Sprintf(`
SELECT
   orders.order_id,
   COUNT(activeleases.lease_id) active,
   COUNT(doneleases.lease_id) done
FROM
  orders

LEFT OUTER JOIN (
  SELECT
    order_id,
    lease_id
  FROM leases
  WHERE expires>NOW() AND done=FALSE
) AS activeleases
ON orders.order_id=activeleases.order_id

LEFT OUTER JOIN (
  SELECT
    order_id,
    lease_id
  FROM leases
  WHERE done=TRUE
) AS doneleases
ON orders.order_id=doneleases.order_id

GROUP BY orders.order_id
HAVING
  %s
ORDER BY
  done DESC,
  active DESC
`, strings.Join(having, " OR ")))
	if err != nil {
		return dbError("Listing orders", err)
	}
	defer rows.Close()
	logRet := &pb.OrdersReply{}
	for rows.Next() {
		if ctx.Err() != nil {
			return err
		}
		e := &pb.OrdersReply{
			Order: &pb.OrderStat{},
		}
		if err := rows.Scan(
			&e.Order.OrderId,
			&e.Order.Active,
			&e.Order.Done,
		); err != nil {
			return dbError("Scanning orders", err)
		}
		if err := stream.Send(e); err != nil {
			return internalError("failed to stream results", "failed to stream results: %v", err)
		}
	}
	if err := rows.Err(); err != nil {
		return dbError("Listing orders", err)
	}

	log.Printf("RPC(Orders)")
	s.rpcLog.Log(ctx, requestID, "dscheduler.Orders", st,
		"github.com/ThomasHabets/qpov/dist/qpov/OrdersRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/OrdersReply", logRet)
	return nil
}

func (s *server) Stats(ctx context.Context, in *pb.StatsRequest) (*pb.StatsReply, error) {
	st := time.Now()
	requestID := uuid.New()
	if err := blockRestrictedAPI(ctx); err != nil {
		return nil, err
	}
	ret := &pb.StatsReply{}
	if in.SchedulingStats {
		ret.SchedulingStats = &pb.SchedulingStats{}
		row := db.QueryRow(`
SELECT
  COUNT(*) total,
  (SELECT COUNT(l.order_id)
    FROM (
      SELECT DISTINCT order_id
      FROM  leases
      WHERE done=FALSE
      AND   expires>NOW()
    ) AS l
  ) active,
  (SELECT COUNT(l.order_id)
    FROM (
      SELECT DISTINCT order_id
      FROM  leases
      WHERE done=TRUE
    ) AS l
  ) done
FROM orders
`)
		if err := row.Scan(
			&ret.SchedulingStats.Orders,
			&ret.SchedulingStats.ActiveOrders,
			&ret.SchedulingStats.DoneOrders,
		); err != nil {
			return nil, dbError("order stats", err)
		}

		row = db.QueryRow(`
SELECT
  (SELECT COUNT(*) FROM leases) total,
  (SELECT COUNT(*) FROM leases WHERE done=FALSE AND expires > NOW()) active,
  (SELECT COUNT(*) FROM leases WHERE done=TRUE) done
`)
		if err := row.Scan(
			&ret.SchedulingStats.Leases,
			&ret.SchedulingStats.ActiveLeases,
			&ret.SchedulingStats.DoneLeases,
		); err != nil {
			return nil, dbError("lease stats", err)
		}
	}
	log.Printf("RPC(Orders)")
	s.rpcLog.Log(ctx, requestID, "dscheduler.Stats", st,
		"github.com/ThomasHabets/qpov/dist/qpov/StatsRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/StatsReply", ret)
	return ret, nil
}

func (s *server) Leases(in *pb.LeasesRequest, stream pb.Scheduler_LeasesServer) error {
	st := time.Now()
	requestID := uuid.New()
	ctx := stream.Context()
	if err := blockRestrictedAPI(ctx); err != nil {
		return err
	}
	ordering := "created"
	if in.Done {
		ordering = "updated"
	}
	rows, err := db.Query(fmt.Sprintf(`
SELECT
  order_id,
  lease_id,
  user_id,
  created,
  updated,
  expires
FROM leases
WHERE done=$1
AND   ($1=TRUE OR expires > NOW())
ORDER BY %s`, ordering), in.Done)
	if err != nil {
		return dbError("Listing leases", err)
	}
	defer rows.Close()
	logRet := &pb.LeasesReply{}

	for rows.Next() {
		if ctx.Err() != nil {
			return err
		}
		var orderID, leaseID string
		var userID *int
		var created, updated, expires time.Time
		if err := rows.Scan(&orderID, &leaseID, &userID, &created, &updated, &expires); err != nil {
			return dbError("Scanning leases", err)
		}
		var order *pb.Order
		if in.Order {
			// TODO: This is very inefficient. Issue just the one query collecting all the data.
			var err error
			order, err = getOrderByID(orderID)
			if err != nil {
				log.Printf("Getting order %q: %v", orderID, err)
			}
		}
		e := &pb.LeasesReply{
			Lease: &pb.Lease{
				OrderId:   orderID,
				LeaseId:   leaseID,
				CreatedMs: created.UnixNano() / 1000000,
				UpdatedMs: updated.UnixNano() / 1000000,
				ExpiresMs: expires.UnixNano() / 1000000,
				Order:     order,
			},
		}
		if userID != nil {
			e.Lease.UserId = int64(*userID)
		}
		if err := stream.Send(e); err != nil {
			return internalError("failed to stream results", "failed to stream results: %v", err)
		}
	}
	if err := rows.Err(); err != nil {
		return dbError("Listing leases", err)
	}

	log.Printf("RPC(Leases)")
	s.rpcLog.Log(ctx, requestID, "dscheduler.Leases", st,
		"github.com/ThomasHabets/qpov/dist/qpov/LeasesRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/LeasesReply", logRet)
	return nil
}

func blockRestrictedAPIInternal(ctx context.Context) error {
	t, ok := transport.StreamFromContext(ctx)
	if !ok {
		return internalError("no stream context", "no stream context")
	}
	if false {
		st := t.ServerTransport()
		log.Printf("Called from %v", st.RemoteAddr())
	}
	a, ok := credentials.FromContext(ctx)
	if !ok {
		return grpc.Errorf(codes.Unauthenticated, "no credentials")
	}
	at, ok := a.(credentials.TLSInfo)
	if !ok {
		return internalError("auth type is not TLSInfo", "auth type is not TLSInfo")
	}
	if len(at.State.PeerCertificates) != 1 {
		return grpc.Errorf(codes.Unauthenticated, "no certificate attached")
	}
	return nil
}

// Verify certificate info and return nil or error suitable for sending to user.
func blockRestrictedAPI(ctx context.Context) error {
	if err := blockRestrictedAPIInternal(ctx); err != nil {
		log.Printf("Restricted API: %v", err)
		return grpc.Errorf(grpc.Code(err), "restricted API called without valid credentials")
	}
	return nil
}

func (s *server) Add(ctx context.Context, in *pb.AddRequest) (*pb.AddReply, error) {
	st := time.Now()
	requestID := uuid.New()
	ownerID, err := getOwnerID(ctx)
	if err != nil {
		log.Printf("RPC(Add): %v", err)
		return nil, grpc.Errorf(codes.Unauthenticated, "authentication required")
	}

	// Look up permission to add.
	// TODO: put this in getOwnerID()
	{
		row := db.QueryRow(`SELECT adding FROM users WHERE user_id=$1`, ownerID)
		var adding bool
		if err := row.Scan(&adding); err == sql.ErrNoRows {
			return nil, internalError("failed looking up user", "failed looking up user")
		} else if err != nil {
			return nil, dbError(fmt.Sprintf("Looking up user %v", ownerID), err)
		}
		if !adding {
			log.Printf("User not allowed to add orders")
			return nil, grpc.Errorf(codes.PermissionDenied, "user not allowed to add orders")
		}
	}

	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO orders(order_id, created, owner, definition) VALUES($1, NOW(), $2,$3)`, id, ownerID, in.OrderDefinition); err != nil {
		return nil, dbError("Inserting order", err)
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
	if flag.NArg() > 0 {
		log.Fatalf("Got extra args on cmdline: %q", flag.Args())
	}

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
