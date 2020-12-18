package rpcclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	pb "github.com/ThomasHabets/qpov/pkg/dist/qpov"
)

var (
	caFile         = flag.String("ca_file", "", "Server CA file. Default to system pool.")
	certFile       = flag.String("cert_file", "", "Client cert file.")
	keyFile        = flag.String("key_file", "", "Client key file.")
	connectTimeout = flag.Duration("connect_timeout", 5*time.Second, "Dial timout.")

	verbose = flag.Bool("rpc_verbose", false, "Print verbose RPC info.")
)

const (
	userAgent = "dmaster"
)

type RPCScheduler struct {
	conn   *grpc.ClientConn
	Client pb.SchedulerClient
}

func (s *RPCScheduler) Close() {
	s.conn.Close()
}

func (s *RPCScheduler) Add(ctx context.Context, order, batchID string) error {
	_, err := s.Client.Add(ctx, &pb.AddRequest{
		OrderDefinition: order,
		BatchId:         batchID,
	})
	return err
}

func NewScheduler(ctx context.Context, addr string) (*RPCScheduler, error) {
	if *verbose {
		log.Infof("Setting GRPC verbose mode")
		grpclog.SetLoggerV2(grpclog.NewLoggerV2WithVerbosity(os.Stderr, os.Stderr, os.Stderr, 99))
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to split host/port out of %q", addr)
	}
	tlsConfig := tls.Config{
		ServerName: host,
	}

	// Load custom CA.
	if *caFile != "" {
		b, err := ioutil.ReadFile(*caFile)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %v", *caFile, err)
		}
		caStr := string(b)
		cp := x509.NewCertPool()
		if ok := cp.AppendCertsFromPEM([]byte(caStr)); !ok {
			return nil, fmt.Errorf("failed to add root CAs")
		}
		tlsConfig.RootCAs = cp
	}

	// Load TLS credentials.
	if *certFile != "" {
		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client keypair %q/%q: %v", *certFile, *keyFile, err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	cr := credentials.NewTLS(&tlsConfig)

	// Connect.
	ctx2, cancel := context.WithTimeout(ctx, *connectTimeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx2, addr,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(cr),
		grpc.WithUserAgent(userAgent))
	if err != nil {
		log.Fatalf("Failed to connect to %q: %v", addr, err)
	}
	return &RPCScheduler{
		conn:   conn,
		Client: pb.NewSchedulerClient(conn),
	}, nil
}
