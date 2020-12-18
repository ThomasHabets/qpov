package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/ThomasHabets/qpov/dist"
	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

var (
	caFile         = flag.String("ca_file", "", "Server CA file.")
	certFile       = flag.String("cert_file", "", "Client cert file.")
	keyFile        = flag.String("key_file", "", "Client key file.")
	connectTimeout = flag.Duration("connect_timeout", 5*time.Second, "Dial timout.")
)

const (
	userAgent = "dmaster"
)

type rpcScheduler struct {
	conn   *grpc.ClientConn
	client pb.SchedulerClient
}

func (s *rpcScheduler) close() {
	//s.conn.Close()
}

func (s *rpcScheduler) add(order, batchID string) error {
	_, err := s.client.Add(context.Background(), &pb.AddRequest{
		OrderDefinition: order,
		BatchId:         batchID,
	})
	return err
}

func newRPCScheduler(addr string) (*rpcScheduler, error) {
	caStr := dist.CacertClass1
	if *caFile != "" {
		b, err := ioutil.ReadFile(*caFile)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %v", *caFile, err)
		}
		caStr = string(b)
	}
	cp := x509.NewCertPool()
	if ok := cp.AppendCertsFromPEM([]byte(caStr)); !ok {
		return nil, fmt.Errorf("failed to add root CAs")
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to split host/port out of %q", addr)
	}
	tlsConfig := tls.Config{
		ServerName: host,
		RootCAs:    cp,
	}
	if *certFile != "" {
		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client keypair %q/%q: %v", *certFile, *keyFile, err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	cr := credentials.NewTLS(&tlsConfig)
	conn, err := grpc.Dial(addr,
		grpc.WithTimeout(*connectTimeout),
		grpc.WithBlock(),
		grpc.WithTransportCredentials(cr),
		grpc.WithUserAgent(userAgent))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	return &rpcScheduler{
		client: pb.NewSchedulerClient(conn),
	}, nil
}
