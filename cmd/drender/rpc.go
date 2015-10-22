package main

import (
	"log"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

type rpcScheduler struct {
	client pb.SchedulerClient
}

func (s *rpcScheduler) get() (string, string, error) {
	r, err := s.client.Get(context.Background(), &pb.GetRequest{})
	return r.LeaseId, r.OrderDefinition, err
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

func newRPCScheduler() scheduler {
	conn, err := grpc.Dial(*schedAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	return &rpcScheduler{client: pb.NewSchedulerClient(conn)}
}
