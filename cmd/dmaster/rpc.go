package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

var (
	caFile = flag.String("ca_file", "", "Server CA file.")
)

const (
	userAgent = "dmaster"
)

type rpcScheduler struct {
	conn   grpc.Conn
	client pb.SchedulerClient
}

func (s *rpcScheduler) close() {
	//s.conn.Close()
}

func (s *rpcScheduler) add(order string) error {
	_, err := s.client.Add(context.Background(), &pb.AddRequest{
		OrderDefinition: order,
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
		log.Fatalf("did not connect: %v", err)
	}
	return &rpcScheduler{
		client: pb.NewSchedulerClient(conn),
	}, nil
}
