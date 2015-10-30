package main

import (
	"fmt"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pb "github.com/ThomasHabets/qpov/dist/qpov"
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

func (s *rpcScheduler) renew(id string, dur time.Duration) error {
	_, err := s.client.Renew(context.Background(), &pb.RenewRequest{
		LeaseId: id,
	})
	return err
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
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithUserAgent(userAgent))
	if err != nil {
		return nil, fmt.Errorf("dialing scheduler %q: %v", addr, err)
	}
	return &rpcScheduler{client: pb.NewSchedulerClient(conn)}, nil
}
