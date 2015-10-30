package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

var (
	caFile = flag.String("ca_file", "", "Server CA file.")
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

func (s *rpcScheduler) done(id string, img, stdout, stderr []byte, j string) error {
	_, err := s.client.Done(context.Background(), &pb.DoneRequest{
		LeaseId:      id,
		Image:        img,
		Stdout:       stdout,
		Stderr:       stderr,
		JsonMetadata: j,
	})
	return err
}

func newRPCScheduler(addr string) (scheduler, error) {
	b, err := ioutil.ReadFile(*caFile)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %v", *caFile, err)
	}
	cp := x509.NewCertPool()
	if ok := cp.AppendCertsFromPEM(b); !ok {
		return nil, fmt.Errorf("failed to add root CAs")
	}
	cr := credentials.NewTLS(&tls.Config{
		RootCAs: cp,
	})
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(cr), grpc.WithUserAgent(userAgent))
	if err != nil {
		return nil, fmt.Errorf("dialing scheduler %q: %v", addr, err)
	}
	return &rpcScheduler{client: pb.NewSchedulerClient(conn)}, nil
}
