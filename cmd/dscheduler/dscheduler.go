// dscheduler is the rendering job scheduler.
//
// It uses postgresql for metadata and stores the output on GCS.
package main

import (
	"compress/gzip"
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
	"strings"
	"sync"
	"time"

	storage "cloud.google.com/go/storage"
	"github.com/ThomasHabets/go-uuid/uuid"
	"github.com/golang/protobuf/proto"
	_ "github.com/lib/pq"
	"golang.org/x/net/context"
	cloudopt "google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	grpcmetadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"

	"github.com/ThomasHabets/qpov/dist"
	pb "github.com/ThomasHabets/qpov/dist/qpov"
	"github.com/ThomasHabets/qpov/dist/rpclog"
)

var (
	db                    dist.DBWrap
	dbConnect             = flag.String("db", "", "")
	addr                  = flag.String("port", ":9999", "Addr to listen to.")
	certFile              = flag.String("cert_file", "", "The TLS cert file")
	anonymous             = flag.Bool("anonymous", true, "Allow anonymous access.")
	keyFile               = flag.String("key_file", "", "The TLS key file")
	clientCAFile          = flag.String("client_ca_file", "", "The client CA file.")
	maxConcurrentStreams  = flag.Int("max_concurrent_streams", 10000, "Max concurrent RPC streams.")
	rpclogDir             = flag.String("rpclog_dir", ".", "RPC log directory.")
	sqlLog                = flag.String("sql_log", "", "Log of SQL statements.")
	minLeaseRenewTime     = flag.Duration("min_lease_renew_time", time.Hour, "Minimum lease renew time.")
	maxLeaseRenewTime     = flag.Duration("max_lease_renew_time", 48*time.Hour, "Minimum lease renew time.")
	defaultLeaseRenewTime = flag.Duration("default_lease_time", time.Hour, "Default lease renew time.")
	oauthClientID         = flag.String("oauth_client_id", "", "Google OAuth Client ID.")
	doRPCLog              = flag.Bool("rpclog", false, "Log all RPCs.")
	keepAliveDuration     = flag.Duration("keepalive", 60*time.Minute, "TCP keepalive time.")

	// Cloud config.
	cloudCredentials    = flag.String("cloud_credentials", "", "Path to JSON file containing credentials.")
	uploadBucketName    = flag.String("cloud_upload_bucket", "", "Google cloud storage bucket name to upload to.")
	downloadBucketNames = flag.String("cloud_download_buckets", "", "Google cloud storage bucket name to read from.")

	errNoCert = errors.New("no cert provided")

	oauthKeys         *OAuthKeys
	validOAuthIssuers = map[string]bool{
		"accounts.google.com":         true,
		"https://accounts.google.com": true,
	}

	googleCloudStorage *storage.Client

	// Driver doesn't support isolation levels natively.
	txopts = &sql.TxOptions{
	//Isolation: sql.LevelSerializable,
	}
)

const (
	propAddresses = "addresses"
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

// parse definition text and turn it into an `Order` proto.
func def2Order(def string) (*pb.Order, error) {
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

func getOrderByID(ctx context.Context, id string) (*pb.Order, error) {
	row := db.QueryRowContext(ctx, `SELECT batch_id,definition FROM orders WHERE order_id=$1`, id)
	var def string
	var batchID sql.NullString
	if err := row.Scan(&batchID, &def); err != nil {
		return nil, err
	}
	ret, err := def2Order(def)
	if err != nil {
		return nil, err
	}
	ret.OrderId = id
	ret.BatchId = batchID.String
	return ret, nil
}

func getOwnerIDByCN(ctx context.Context, cn string) (int, error) {
	row := db.QueryRowContext(ctx, `SELECT user_id FROM certs WHERE cn=$1`, cn)
	var ownerID int
	if err := row.Scan(&ownerID); err == sql.ErrNoRows {
		return 0, fmt.Errorf("client cert not assigned to any user")
	} else if err != nil {
		return 0, fmt.Errorf("failed looking up cert: %v", err)
	}
	return ownerID, nil
}

func getPeerCert(ctx context.Context) (*x509.Certificate, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errNoCert
	}
	at, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, fmt.Errorf("auth type is not TLSInfo")
	}
	if len(at.State.PeerCertificates) != 1 {
		return nil, errNoCert
	}
	cert := at.State.PeerCertificates[0]
	return cert, nil
}

type server struct {
	rpcLog *rpclog.Logger
}

func clientAddressFromContext(ctx context.Context) (string, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", fmt.Errorf("no peer associated with context")
	}
	client, _, _ := net.SplitHostPort(p.Addr.String())
	return client, nil
}

type user struct {
	userID       int
	comment      string
	oauthSubject string
	adding       bool
	delegate     bool
	addresses    bool
	via          *user
}

func readUserByID(ctx context.Context, userID int) (*user, error) {
	return readUserBySomething(ctx, "user_id", userID)
}
func readUserByOAuth(ctx context.Context, sub string) (*user, error) {
	return readUserBySomething(ctx, "oauth_subject", sub)
}

func readUserBySomething(ctx context.Context, column string, value interface{}) (*user, error) {
	u := &user{}
	var comment, oauthSubject sql.NullString
	if err := db.QueryRowContext(ctx, fmt.Sprintf(`
SELECT
  user_id,
  delegate,
  addresses,
  adding,
  comment,
  oauth_subject
FROM
  users
WHERE
  %s=$1
`, column), value).Scan(&u.userID, &u.delegate, &u.addresses, &u.adding, &comment, &oauthSubject); err != nil {
		return nil, err
	}
	u.comment = comment.String
	u.oauthSubject = oauthSubject.String
	return u, nil
}

// Get end-user and any intermediary (webserver).
//
// TODO: This is 4-6 SQL queries for every call.
// * One for TLS cert
// * One for TLS user
// * Cookie lookup
// * SELECT/UPDATE/SELECT for cookied user if they're not in the database.
func getUser(ctx context.Context) (*user, error) {
	var u *user
	// First use TLS credentials.
	// No TLS credentials -> not authenticated.
	{
		cert, err := getPeerCert(ctx)
		if err != nil {
			return nil, err
		}

		id, err := getOwnerIDByCN(ctx, cert.Subject.CommonName)
		if err != nil {
			return nil, err
		}
		u, err = readUserByID(ctx, id)
		if err != nil {
			return nil, err
		}
	}

	// Now check if it's just delegated.
	if u.delegate {
		v, err := getRPCMetadata(ctx, "http.cookie")
		if err != nil {
			// No cookie. Act as webserver.
			return u, nil
		}
		j, err := getJWTFromCookie(ctx, v)
		if err != nil {
			// Bad cookie. Act as webserver.
			return u, nil
		}
		email, sub, err := oauthKeys.VerifyJWT(j)
		if err != nil {
			// Bad JWT. Act as webserver.
			return u, nil
		}
		nu, err := readUserByOAuth(ctx, sub)
		if err != nil {
			// OAuth subject valid but unknown. Associate with email if possible.
			if err := setUserOAuthByEmail(ctx, email, sub); err != nil {
				// Logged in but not as anyone in the database.
				return u, nil
			}
		}
		nu, err = readUserByOAuth(ctx, sub)
		if err != nil {
			// Not in database. Fill in what we can.
			nu = &user{
				oauthSubject: sub,
			}
		}
		nu.via = u
		u = nu
	}
	return u, nil
}

func setUserOAuthByEmail(ctx context.Context, email, sub string) error {
	_, err := db.ExecContext(ctx, `
UPDATE users
SET    oauth_subject=$2
WHERE  email=$1
AND    oauth_subject IS NULL
`, email, sub)
	return err
}

func mintCert(u *user) ([]byte, error) {
	return nil, grpc.Errorf(codes.Unimplemented, "minting certs not implemented yet")
}

func (s *server) Certificate(ctx context.Context, in *pb.CertificateRequest) (*pb.CertificateReply, error) {
	if err := blockRestrictedAPI(ctx); err != nil {
		return nil, err
	}
	user, err := getUser(ctx)
	if err != nil {
		log.Printf("RPC(Certificate): getUser(): %v", err)
		return nil, grpc.Errorf(codes.Unauthenticated, "authentication required")
	}
	if user.delegate {
		log.Printf("Not minting cert for webserver")
		return nil, grpc.Errorf(codes.Unauthenticated, "authentication required")
	}
	b, err := mintCert(user)
	if err != nil {
		log.Printf("Failed to mint cert for user %d: %v", user.userID, err)
		return nil, cleanError(err, codes.Internal, "failed to mint cert")
	}
	return &pb.CertificateReply{Pem: b}, nil
}

func (s *server) Login(ctx context.Context, in *pb.LoginRequest) (*pb.LoginReply, error) {
	if err := blockRestrictedAPI(ctx); err != nil {
		return nil, err
	}

	if _, _, err := oauthKeys.VerifyJWT(in.Jwt); err != nil {
		log.Printf("RPC(Login): %v", err)
		return nil, fmt.Errorf("failed to verify jwt")
	}
	tx, err := db.BeginTx(ctx, txopts)
	if err != nil {
		return nil, dbError("begin transaction", err)
	}
	defer tx.Rollback()
	// Driver doesn't support isolation levels natively.
	if true {
		if _, err := db.ExecContext(ctx, "SET TRANSACTION ISOLATION LEVEL SERIALIZABLE"); err != nil {
			return nil, dbError("set isolation level serializable", err)
		}
	}

	// Reuse cookie if present and is UUID-shaped.
	c := in.Cookie
	if c == "" || uuid.Parse(c) == nil {
		c = uuid.New()
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cookies WHERE cookie=$1`, c); err != nil {
		return nil, dbError("clearing old cookie", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO cookies(cookie, jwt) VALUES($1, $2)`, c, in.Jwt); err != nil {
		return nil, dbError("saving cookie", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, dbError("Committing transaction", err)
	}
	return &pb.LoginReply{Cookie: c}, nil
}

func (s *server) Logout(ctx context.Context, in *pb.LogoutRequest) (*pb.LogoutReply, error) {
	if _, err := db.ExecContext(ctx, `DELETE FROM cookies WHERE cookie=$1`, in.Cookie); err != nil {
		return nil, dbError("clearing cookie", err)
	}
	return &pb.LogoutReply{}, nil
}

func (s *server) CheckCookie(ctx context.Context, in *pb.CheckCookieRequest) (*pb.CheckCookieReply, error) {
	var j string
	if err := db.QueryRowContext(ctx, `SELECT jwt FROM cookies WHERE cookie=$1`, in.Cookie).Scan(&j); err != nil {
		return nil, dbError("searching for cookie", err)
	}
	if _, _, err := oauthKeys.VerifyJWT(j); err != nil {
		return nil, grpc.Errorf(codes.Unauthenticated, "cookie known but not valid at this time")
	}
	return &pb.CheckCookieReply{}, nil
}

func (s *server) Get(ctx context.Context, in *pb.GetRequest) (*pb.GetReply, error) {
	st := time.Now()
	requestID := uuid.New()

	tx, err := db.BeginTx(ctx, txopts)
	if err != nil {
		return nil, dbError("begin transaction", err)
	}
	defer tx.Rollback()
	// Driver doesn't support isolation levels natively.
	if true {
		if _, err := db.ExecContext(ctx, "SET TRANSACTION ISOLATION LEVEL SERIALIZABLE"); err != nil {
			return nil, dbError("set isolation level serializable", err)
		}
	}

	var orderID, def string
	// TODO: Currently using batch.ctime() as priority. Should have separate thing?
	if err := tx.QueryRowContext(ctx, `
SELECT
  orders.order_id,
  orders.definition
FROM
  orders
JOIN (
  SELECT          CAST(MIN(CAST(orders.order_id AS TEXT)) AS UUID) AS order_id
  FROM            orders
  LEFT OUTER JOIN batch  ON orders.batch_id=batch.batch_id
  LEFT OUTER JOIN (
    SELECT DISTINCT order_id, MAX(CAST(done AS INT))>0 AS done, MAX(expires) AS expires
    FROM            leases
    GROUP BY        order_id
  ) active_or_done
    ON            orders.order_id=active_or_done.order_id
  WHERE           active_or_done.order_id IS NULL        -- Not attempted at all yet.
  OR              (active_or_done.done = FALSE AND active_or_done.expires < NOW()) -- Not done and none is already active.
  GROUP BY        batch.batch_id
  ORDER BY        batch.ctime
  LIMIT 1
) next_order
ON next_order.order_id=orders.order_id
`).Scan(&orderID, &def); err == sql.ErrNoRows {
		return nil, grpc.Errorf(codes.NotFound, "nothing to do")
	} else if err != nil {
		return nil, dbError("Scanning order", err)
	}

	var ownerID *int
	if user, err := getUser(ctx); err == nil {
		ownerID = &user.userID
	} else if err != errNoCert {
		log.Printf("Getting user ID: %v", err)
	}

	lease := uuid.New()
	clientAddress, _ := clientAddressFromContext(ctx)
	if _, err := tx.ExecContext(ctx, `
INSERT INTO leases(
  lease_id, done, order_id,
  user_id, client, hostname,
  created, updated, expires
)
VALUES(
  $1, false, $2,
  $3, $4, $5,
  NOW(), NOW(), $6
)
`, lease, orderID, ownerID, clientAddress, getRPCMetadataSQL(ctx, "hostname"), time.Now().Add(*defaultLeaseRenewTime)); err != nil {
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

func getRPCMetadataSQL(ctx context.Context, s string) sql.NullString {
	var ret sql.NullString
	h, err := getRPCMetadata(ctx, s)
	if err != nil {
		log.Printf("Couldn't get metadata %q: %v", s, err)
		return ret
	}
	ret.Valid = true
	ret.String = h
	return ret
}

func getRPCMetadata(ctx context.Context, s string) (string, error) {
	md, ok := grpcmetadata.FromContext(ctx)
	if !ok {
		return "", fmt.Errorf("could not get RPC metadata")
	}
	v, _ := md[s]
	if len(v) == 0 {
		return "", fmt.Errorf("no %q in RPC metadata", s)
	}
	return v[0], nil
}

// renew is the backend for Renew().
func (s *server) renew(ctx context.Context, lease, address string, secs int32) (time.Time, error) {
	if secs <= 0 {
		secs = int32(defaultLeaseRenewTime.Seconds())
	}
	if secs > int32(maxLeaseRenewTime.Seconds()) {
		secs = int32(maxLeaseRenewTime.Seconds())
	}
	if secs < int32(minLeaseRenewTime.Seconds()) {
		secs = int32(minLeaseRenewTime.Seconds())
	}

	n := time.Now().Add(time.Duration(int64(time.Second) * int64(secs)))
	if _, err := db.ExecContext(ctx, `
UPDATE leases
SET
    updated=NOW(),
    expires=$1,
    client=$3,
    hostname=$4
WHERE lease_id=$2
AND   done=FALSE
AND   failed=FALSE
`, n, lease, address, getRPCMetadataSQL(ctx, "hostname")); err != nil {
		return time.Now(), dbError("Updating lease", err)
	}
	return n, nil
}

func (s *server) Renew(ctx context.Context, in *pb.RenewRequest) (*pb.RenewReply, error) {
	st := time.Now()
	requestID := uuid.New()

	client, _ := clientAddressFromContext(ctx)

	n, err := s.renew(ctx, in.LeaseId, client, in.ExtendSec)
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

func (s *server) Failed(ctx context.Context, in *pb.FailedRequest) (*pb.FailedReply, error) {
	st := time.Now()
	requestID := uuid.New()

	if _, err := db.ExecContext(ctx, `
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

func leaseDone(ctx context.Context, id string) (bool, bool, error) {
	row := db.QueryRowContext(ctx, `SELECT done, failed FROM leases WHERE lease_id=$1`, id)
	var done, failed bool
	if err := row.Scan(&done, &failed); err != nil {
		return false, false, err
	}
	return done, failed, nil
}

// Get the correct metadata and return it both as proto and string.
// TODO: Once all clients use proto version this function will not be needed.
func getMetadata(in *pb.DoneRequest) (*pb.RenderingMetadata, string, error) {
	var stats *pb.RenderingMetadata
	if in.Metadata != nil {
		stats = in.Metadata
	} else {
		var err error
		stats, err = dist.ParseLegacyJSON([]byte(in.JsonMetadata))
		if err != nil {
			return nil, "", fmt.Errorf("parsing metadata: %v", err)
		}
	}
	b, err := json.Marshal(stats)
	if err != nil {
		return nil, "", fmt.Errorf("marshaling to newJSON: %v", err)
	}
	return stats, string(b), nil
}

// Save results to Google Cloud Storage under gs://<bucket>/<type>/<leaseID>/filename.{png,meta.pb.gz}
func (s *server) saveToCloud(ctx context.Context, in *pb.DoneRequest, realMeta *pb.RenderingMetadata, base string, batch uuid.UUID) error {
	dir := ""
	if batch != nil {
		dir = path.Join(dir, "batch", batch.String())
	} else {
		dir = path.Join(dir, "single")
	}
	dir = path.Join(dir, in.LeaseId)
	var wg sync.WaitGroup
	wg.Add(2)

	var imageErr error
	go func() {
		defer wg.Done()
		fn := path.Join(dir, base+".png")
		obj := googleCloudStorage.Bucket(*uploadBucketName).Object(fn)
		if _, err := obj.Attrs(ctx); err == nil {
			log.Printf("File already exist, skipping %q %q", *uploadBucketName, fn)
			return
		} else if err != storage.ErrObjectNotExist {
			log.Printf("Failed to check if %q %q already exists: %v", err)
		}
		w := obj.NewWriter(ctx)
		defer func() {
			if imageErr == nil {
				imageErr = w.Close()
			} else {
				w.CloseWithError(imageErr)
			}
		}()
		_, imageErr = w.Write(in.Image)
	}()

	var metaErr error
	go func() {
		defer wg.Done()
		fn := path.Join(dir, base+".meta.pb.gz")
		obj := googleCloudStorage.Bucket(*uploadBucketName).Object(fn)
		if _, err := obj.Attrs(ctx); err == nil {
			log.Printf("File already exist, skipping %q %q", *uploadBucketName, fn)
			return
		} else if err != storage.ErrObjectNotExist {
			log.Printf("Failed to check if %q %q already exists: %v", err)
		}
		w := obj.NewWriter(ctx)
		defer func() {
			if metaErr == nil {
				metaErr = w.Close()
			} else {
				w.CloseWithError(metaErr)
			}
		}()
		zw := gzip.NewWriter(w)
		defer func() {
			e := zw.Close()
			if metaErr == nil {
				metaErr = e
			}
		}()
		// TODO: Client should send stdout/stderr in metadata directly, and this won't be needed.
		d := proto.Clone(realMeta).(*pb.RenderingMetadata)
		if len(d.Stdout) == 0 {
			d.Stdout = in.Stdout
		}
		if len(d.Stderr) == 0 {
			d.Stderr = in.Stderr
		}
		data, err := proto.Marshal(d)
		if err != nil {
			metaErr = err
			return
		}
		_, metaErr = zw.Write(data)
	}()

	wg.Wait()
	if imageErr != nil {
		return imageErr
	}
	return metaErr
}

// getOrderDestByLeastID returns the filename and batch ID of an order.
func getOrderDestByLeaseID(ctx context.Context, leaseID string) (string, uuid.UUID, error) {
	row := db.QueryRowContext(ctx, `
SELECT
  orders.definition,
  orders.batch_id
FROM orders
JOIN leases ON orders.order_id=leases.order_id
WHERE lease_id=$1`, leaseID)
	var def string
	var b sql.NullString
	if err := row.Scan(&def, &b); err != nil {
		return "", nil, err
	}
	var order dist.Order
	if err := json.Unmarshal([]byte(def), &order); err != nil {
		return "", nil, err
	}
	return order.File, uuid.Parse(b.String), nil
}

func (s *server) Done(ctx context.Context, in *pb.DoneRequest) (*pb.DoneReply, error) {
	st := time.Now()
	requestID := uuid.New()

	var client string
	{
		if p, ok := peer.FromContext(ctx); ok {
			client, _, _ = net.SplitHostPort(p.Addr.String())
		}
	}

	// First give us time to receive the data.
	if _, err := s.renew(ctx, in.LeaseId, client, -1); err != nil {
		log.Printf("RPC(Done): Failed to renew before Done'ing: %v", err)
	}
	isDone, isFailed, err := leaseDone(ctx, in.LeaseId)
	if err != nil {
		return nil, dbError(fmt.Sprintf("Failed looking up lease %q", in.LeaseId), err)
	}
	if isDone {
		return nil, grpc.Errorf(codes.AlreadyExists, "lease already done: %q", in.LeaseId)
	}
	if isFailed {
		return nil, grpc.Errorf(codes.AlreadyExists, "lease already failed: %q", in.LeaseId)
	}

	// Fetch the order ID so we can assemble the path.
	file, batch, err := getOrderDestByLeaseID(ctx, in.LeaseId)
	if err != nil {
		log.Printf("RPC(Done): Can't find order with lease %q: %v", in.LeaseId, err)
		return nil, grpc.Errorf(codes.NotFound, "unknown lease %q", in.LeaseId)
	}

	// Create metadata to store in DB from oldJSON or proto.
	realMeta, newStats, err := getMetadata(in)
	if err != nil {
		log.Printf("Warning: Failed to get metadata: %v", err)
		// newStats defaults to being empty.
	}

	// Upload to (Google) cloud.
	{
		basefile := strings.TrimSuffix(file, path.Ext(file))
		if err := s.saveToCloud(ctx, in, realMeta, basefile, batch); err != nil {
			return nil, internalError("failed to save to cloud", "failed to save to cloud: %v", err)
		}
	}

	// Mark as completed in database.
	if _, err := db.ExecContext(ctx, `
UPDATE leases
SET
    done=TRUE,
    updated=NOW(),
    metadata=$2
WHERE lease_id=$1
AND   done=FALSE
AND   failed=FALSE
`, in.LeaseId, newStats); err != nil {
		return nil, dbError("Marking done", err)
	}

	log.Printf("RPC(Done): Lease: %q", in.LeaseId)
	ret := &pb.DoneReply{}
	s.rpcLog.Log(ctx, requestID, "dscheduler.Done", st,
		"github.com/ThomasHabets/qpov/dist/qpov/DoneRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/DoneReply", ret)
	return ret, nil
}

func resultsGCS(ctx context.Context, batch uuid.UUID, leaseID string, base string, stream pb.Scheduler_ResultServer) error {
	dir := ""
	if batch != nil {
		dir = path.Join(dir, "batch", batch.String())
	} else {
		dir = path.Join(dir, "single")
	}
	dir = path.Join(dir, leaseID)
	for _, bucketName := range strings.Split(*downloadBucketNames, ",") {
		r, err := googleCloudStorage.Bucket(bucketName).Object(path.Join(dir, base+".png")).NewReader(ctx)
		if err == storage.ErrObjectNotExist {
			// Try next bucket.
			continue
		} else if err != nil {
			return internalError("failed to stream results", "failed to open a reader: %v", err)
		}
		const chunkSize = 1000
		for {
			buf := make([]byte, chunkSize, chunkSize)
			n, err := r.Read(buf)
			if err == io.EOF {
				break
			} else if err != nil {
				return internalError("failed to stream results", "failed to stream results while reading: %v", err)
			}
			if err := stream.Send(&pb.ResultReply{Data: buf[0:n]}); err != nil {
				return internalError("failed to stream results", "failed to stream results while sending: %v", err)
			}
		}
		return nil
	}
	return grpc.Errorf(codes.NotFound, "result batch %q lease %q not found", batch, leaseID)
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
		file, batch, err := getOrderDestByLeaseID(ctx, in.LeaseId)
		if err != nil {
			log.Printf("Can't find order with lease %q: %v", in.LeaseId, err)
			return grpc.Errorf(codes.NotFound, "unknown lease %q", in.LeaseId)
		}

		base := strings.TrimSuffix(file, ".pov")
		if err := resultsGCS(ctx, batch, in.LeaseId, base, stream); err != nil {
			return err
		}
	}

	log.Printf("RPC(Result) Lease %s", in.LeaseId)
	s.rpcLog.Log(ctx, requestID, "dscheduler.Result", st,
		"github.com/ThomasHabets/qpov/dist/qpov/ResultRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/ResultReply", &ret)
	return nil
}

func (s *server) Order(ctx context.Context, in *pb.OrderRequest) (*pb.OrderReply, error) {
	st := time.Now()
	requestID := uuid.New()
	if len(in.OrderId) != 1 {
		return nil, grpc.Errorf(codes.Unimplemented, "getting multiple orders not implemented yet")
	}
	o, err := getOrderByID(ctx, in.OrderId[0])
	if err == sql.ErrNoRows {
		return nil, grpc.Errorf(codes.NotFound, "order not found")
	} else if err != nil {
		return nil, dbError("getting order", err)
	}

	ret := &pb.OrderReply{Order: []*pb.Order{o}}
	log.Printf("RPC(Order) %s", in.OrderId)
	s.rpcLog.Log(ctx, requestID, "dscheduler.Order", st,
		"github.com/ThomasHabets/qpov/dist/qpov/OrderRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/OrderReply", ret)
	return ret, nil
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
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`
-- Presentation query.
SELECT
   orders.order_id,
   orders.batch_id,
   COUNT(activeleases.lease_id) active,
   COUNT(doneleases.lease_id) done
FROM
  orders

-- Active leases.
LEFT OUTER JOIN (
  SELECT
    order_id,
    lease_id
  FROM leases
  WHERE expires>NOW() AND done=FALSE
) AS activeleases
ON orders.order_id=activeleases.order_id

-- Done leases.
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
		var batchID sql.NullString
		if err := rows.Scan(
			&e.Order.OrderId,
			&batchID,
			&e.Order.Active,
			&e.Order.Done,
		); err != nil {
			return dbError("Scanning orders", err)
		}
		e.Order.BatchId = batchID.String
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
		row := db.QueryRowContext(ctx, `
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

		row = db.QueryRowContext(ctx, `
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
	log.Printf("RPC(Stats)")
	s.rpcLog.Log(ctx, requestID, "dscheduler.Stats", st,
		"github.com/ThomasHabets/qpov/dist/qpov/StatsRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/StatsReply", ret)
	return ret, nil
}

func verifyUUID(s string) bool {
	return uuid.Parse(s) != nil
}

func (s *server) Lease(ctx context.Context, in *pb.LeaseRequest) (*pb.LeaseReply, error) {
	st := time.Now()
	requestID := uuid.New()
	if err := blockRestrictedAPI(ctx); err != nil {
		return nil, err
	}
	if err := blockRestrictedUser(ctx); err != nil {
		return nil, err
	}

	if !verifyUUID(in.LeaseId) {
		return nil, grpc.Errorf(codes.InvalidArgument, "%q is not a valid UUID", in.LeaseId)
	}

	row := db.QueryRowContext(ctx, `
SELECT lease_id, order_id, done, failed, created, updated, expires, metadata
FROM leases
WHERE lease_id=$1`, in.LeaseId)
	ret := &pb.LeaseReply{
		Lease: &pb.Lease{},
	}
	var ctime, mtime, dtime time.Time
	var metadata *string
	if err := row.Scan(
		&ret.Lease.LeaseId,
		&ret.Lease.OrderId,
		&ret.Lease.Done,
		&ret.Lease.Failed,
		&ctime, &mtime, &dtime,
		&metadata); err == sql.ErrNoRows {
		return nil, grpc.Errorf(codes.NotFound, "lease %q not found", in.LeaseId)
	} else if err != nil {
		return nil, dbError("Getting lease", err)
	}
	if metadata != nil {
		t := &pb.RenderingMetadata{}
		if err := json.Unmarshal([]byte(*metadata), t); err != nil {
			log.Printf("Failed to parse stats in db for lease %q: %v", in.LeaseId, err)
		} else {
			ret.Lease.Metadata = t
		}
	}

	log.Printf("RPC(Lease) %s", in.LeaseId)
	s.rpcLog.Log(ctx, requestID, "dscheduler.Lease", st,
		"github.com/ThomasHabets/qpov/dist/qpov/LeaseRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/LeaseReply", ret)
	return ret, nil
}

func (s *server) Leases(in *pb.LeasesRequest, stream pb.Scheduler_LeasesServer) error {
	log.Printf("RPC(Leases)")
	st := time.Now()
	requestID := uuid.New()
	ctx := stream.Context()
	if err := blockRestrictedAPI(ctx); err != nil {
		return err
	}

	metadataCol := "NULL"
	if in.Metadata {
		metadataCol = "metadata"
	}
	ordering := "leases.created"
	if in.Done {
		ordering = "leases.updated"
	}
	q := fmt.Sprintf(`
SELECT
  orders.order_id,
  orders.batch_id,
  orders.definition,
  lease_id,
  user_id,
  leases.created,
  updated,
  expires,
  client,
  hostname,
  leases.done,
  %s
FROM leases
JOIN orders ON orders.order_id=leases.order_id
WHERE done=$1
AND   ($1=TRUE OR expires > NOW())
AND   leases.updated > $2
ORDER BY %s`, metadataCol, ordering)
	rows, err := db.QueryContext(ctx, q, in.Done, time.Unix(in.Since, 0))
	if err != nil {
		return dbError("Listing leases", err)
	}
	defer rows.Close()
	logRet := &pb.LeasesReply{}

	user, err := getUser(ctx)
	if err != nil {
		log.Printf("Getting user: %v", err)
		return fmt.Errorf("error getting user")
	}
	for rows.Next() {
		if ctx.Err() != nil {
			return err
		}
		var orderID, leaseID string
		var batchID sql.NullString
		var userID *int
		var created, updated, expires time.Time
		var metadata *string
		var def string
		var client, clientHostname sql.NullString
		var done bool
		if err := rows.Scan(&orderID, &batchID, &def, &leaseID, &userID, &created, &updated, &expires, &client, &clientHostname, &done, &metadata); err != nil {
			return dbError("Scanning leases", err)
		}
		var order *pb.Order
		if in.Order {
			var err error
			order, err = def2Order(def)
			if err != nil {
				log.Printf("Getting order %q: %v", orderID, err)
			} else {
				order.BatchId = batchID.String
			}
		}
		e := &pb.LeasesReply{
			Lease: &pb.Lease{
				OrderId:   orderID,
				CreatedMs: created.UnixNano() / 1000000,
				UpdatedMs: updated.UnixNano() / 1000000,
				ExpiresMs: expires.UnixNano() / 1000000,
				Order:     order,
			},
		}
		if done {
			e.Lease.LeaseId = leaseID
			// Only set some fields for some users.
		}
		if user.addresses {
			e.Lease.Address = client.String
			e.Lease.LeaseId = leaseID
			e.Lease.Hostname = clientHostname.String
		}
		if userID != nil {
			e.Lease.UserId = int64(*userID)
		}
		if metadata != nil {
			t := &pb.RenderingMetadata{}
			if err := json.Unmarshal([]byte(*metadata), t); err != nil {
				log.Printf("Failed to unmarshal JSON stats for lease %q: %v", leaseID, err)
			} else {
				e.Lease.Metadata = t
			}
		}
		if err := stream.Send(e); err != nil {
			return internalError("failed to stream results", "failed to stream results: %v", err)
		}
	}
	if err := rows.Err(); err != nil {
		return dbError("Listing leases", err)
	}

	log.Printf("RPC(Leases) %v", time.Since(st))
	s.rpcLog.Log(ctx, requestID, "dscheduler.Leases", st,
		"github.com/ThomasHabets/qpov/dist/qpov/LeasesRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/LeasesReply", logRet)
	return nil
}

func getJWTFromCookie(ctx context.Context, v string) (string, error) {
	var j string
	if err := db.QueryRowContext(ctx, `SELECT jwt FROM cookies WHERE cookie=$1`, v).Scan(&j); err != nil {
		return "", err
	}
	return j, nil
}

// Block access for end-users that don't have the right cookie set.
// TODO: remove this function since it just calls another?
func blockRestrictedUser(ctx context.Context) error {
	nope := grpc.Errorf(codes.Unauthenticated, "need per-user credentials for this resource")
	user, err := getUser(ctx)
	if err != nil {
		return nope
	}
	if !user.addresses {
		return nope
	}
	return nil
}

// Verify certificate info and return nil or error suitable for sending to user.
// This blocks RPCs completely from a peer, and thus does not consider web oauth `delegation`.
// For that, see `blockRestrictedUser`.
func blockRestrictedAPI(ctx context.Context) error {
	if _, err := getPeerCert(ctx); err != nil {
		log.Printf("Restricted API called: %v", err)
		return grpc.Errorf(codes.Unauthenticated, "restricted API called without valid credentials")
	}
	return nil
}

func (s *server) Add(ctx context.Context, in *pb.AddRequest) (*pb.AddReply, error) {
	st := time.Now()
	requestID := uuid.New()
	user, err := getUser(ctx)
	if err != nil {
		log.Printf("RPC(Add): Looking up user: %v", err)
		return nil, grpc.Errorf(codes.Unauthenticated, "authentication required")
	}

	// Look up permission to add.
	if !user.adding {
		log.Printf("RPC(Add): User not allowed to add orders")
		return nil, grpc.Errorf(codes.PermissionDenied, "user not allowed to add orders")
	}

	var batchID *string
	if in.BatchId != "" {
		batchID = &in.BatchId
	}
	id := uuid.New()
	if _, err := db.ExecContext(ctx, `
INSERT INTO orders(
    order_id,
    batch_id,
    created,
    owner,
    definition
)
VALUES(
    $1,
    $2,
    NOW(),
    $3,
    $4
)`, id, batchID, user.userID, in.OrderDefinition); err != nil {
		return nil, dbError("Inserting order", err)
	}
	log.Printf("RPC(Add): Order: %q", id)

	ret := &pb.AddReply{OrderId: id}
	s.rpcLog.Log(ctx, requestID, "dscheduler.Add", st,
		"github.com/ThomasHabets/qpov/dist/qpov/AddRequest", in,
		nil, "github.com/ThomasHabets/qpov/dist/qpov/AddReply", ret)
	return ret, nil
}

type keepAliveListener struct {
	l net.Listener
}

func (k *keepAliveListener) Accept() (net.Conn, error) {
	c, err := k.l.Accept()
	if err == nil {
		t := c.(*net.TCPConn)
		t.SetKeepAlive(true)
		t.SetKeepAlivePeriod(*keepAliveDuration)
	}
	return c, err
}

func (k *keepAliveListener) Close() error {
	return k.l.Close()
}

func (k *keepAliveListener) Addr() net.Addr {
	return k.l.Addr()
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)
	flag.Parse()
	if flag.NArg() > 0 {
		log.Fatalf("Got extra args on cmdline: %q", flag.Args())
	}

	if *uploadBucketName == "" {
		log.Fatalf("-cloud_upload_bucket must be specified")
	}
	if *downloadBucketNames == "" {
		log.Fatalf("-cloud_download_buckets must be specified")
	}

	ctx := context.Background()

	// Set up OAuth verifier.
	{

		var err error
		oauthKeys, err = NewOAuthKeys(ctx, OAuthURLGoogle)
		if err != nil {
			log.Fatalf("OAuth setup failure: %v", err)
		}
		go func() {
			for {
				time.Sleep(time.Hour)
				if err := oauthKeys.Update(ctx); err != nil {
					log.Printf("Updating OAuthKeys: %v", err)
				}
			}
		}()
	}

	// Connect to GCS.
	{
		var err error
		googleCloudStorage, err = storage.NewClient(ctx, cloudopt.WithServiceAccountFile(*cloudCredentials))
		if err != nil {
			log.Fatal(err)
		}
		defer googleCloudStorage.Close()
	}

	var sqlLogf io.Writer
	{
		fn := *sqlLog
		if fn == "" {
			fn = "/dev/null"
		}
		f, err := os.Create(fn)
		if err != nil {
			log.Fatalf("Opening sql log file %q: %v", fn, err)
		}
		defer f.Close()
		sqlLogf = f
	}

	// Connect to database.
	var err error
	{
		t, err := sql.Open("postgres", *dbConnect)
		if err != nil {
			log.Fatal(err)
		}
		if err := t.PingContext(ctx); err != nil {
			log.Fatalf("db ping: %v", err)
		}
		db = dist.NewDBWrap(t, log.New(sqlLogf, "", log.LstdFlags))
	}

	// Listen to RPC port.
	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	lis = &keepAliveListener{lis}
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
		if !*anonymous {
			t.ClientAuth = tls.RequireAndVerifyClientCert
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(t)))
	}

	// Set up RPC logger.
	var l *rpclog.Logger
	if *doRPCLog {
		now := time.Now()
		fin, err := os.Create(path.Join(*rpclogDir, fmt.Sprintf("rpclog.%d.in.gob", now.Unix())))
		if err != nil {
			log.Fatalf("Opening rpclog: %v", err)
		}
		fout, err := os.Create(path.Join(*rpclogDir, fmt.Sprintf("rpclog.%d.out.gob", now.Unix())))
		if err != nil {
			log.Fatalf("Opening rpclog: %v", err)
		}
		l = rpclog.New(fin, fout)
	} else {
		l = rpclog.New(ioutil.Discard, ioutil.Discard)
	}

	// Create RPC server.
	s := grpc.NewServer(opts...)
	serv := server{rpcLog: l}
	pb.RegisterSchedulerServer(s, &serv)
	pb.RegisterCookieMonsterServer(s, &serv)

	// Run forever.
	log.Printf("Running...")
	log.Fatal(s.Serve(lis))
}
