package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/ThomasHabets/qpov/dist"
	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

var (
	caFile   = flag.String("ca_file", "", "Server CA file.")
	certFile = flag.String("cert_file", "", "Client cert file.")
	keyFile  = flag.String("key_file", "", "Client key file.")

	forwardRPCKeys []string
)

const (
	userAgent = "drender"
)

type rpcScheduler struct {
	client pb.SchedulerClient
}

func (s *rpcScheduler) get() (string, string, error) {
	r, err := s.client.Get(context.Background(), &pb.GetRequest{})
	if err != nil {
		return "", "", err
	}
	return r.LeaseId, r.OrderDefinition, nil
}

func (s *rpcScheduler) renew(id string, dur time.Duration) (time.Time, error) {
	r, err := s.client.Renew(context.Background(), &pb.RenewRequest{
		LeaseId:   id,
		ExtendSec: int32(dur.Seconds()),
	})
	if err != nil {
		return time.Now(), err
	}
	n := time.Unix(r.NewTimeoutSec, 0)
	if r.NewTimeoutSec == 0 {
		n = time.Now().Add(dur) // Just assume duration renewal was accepted.
	}
	return n, nil
}

func (s *rpcScheduler) done(id string, img, stdout, stderr []byte, meta *pb.RenderingMetadata) error {
	_, err := s.client.Done(context.Background(), &pb.DoneRequest{
		LeaseId:  id,
		Image:    img,
		Stdout:   stdout,
		Stderr:   stderr,
		Metadata: meta,
	})
	return err
}

func newRPCScheduler(addr string) (scheduler, error) {
	caStr := dist.CacertClass1
	if *caFile != "" {
		b, err := ioutil.ReadFile(*caFile)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %v", *caFile, err)
		}
		caStr = string(b)
	}

	// Root CA.
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

	// Client Cert.
	if *certFile != "" {
		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client keypair %q/%q: %v", *certFile, *keyFile, err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	cr := credentials.NewTLS(&tlsConfig)
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(cr),
		grpc.WithPerRPCCredentials(dist.NewPerRPC(forwardRPCKeys)),
		grpc.WithUserAgent(userAgent),
	)
	if err != nil {
		return nil, fmt.Errorf("dialing scheduler %q: %v", addr, err)
	}
	return &rpcScheduler{client: pb.NewSchedulerClient(conn)}, nil
}
