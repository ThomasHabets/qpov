package main

import (
	"log"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

type rpcScheduler struct {
	client pb.SchedulerClient
}

func (s *rpcScheduler) get() (string, string, error) {
	r, err := s.client.Get(context.Background(), &pb.GetRequest{})
	return r.GetLeaseId(), r.GetOrderDefinition(), err
}

func (s *rpcScheduler) renew(id string, dur time.Duration) error {
	_, err := s.client.Renew(context.Background(), &pb.RenewRequest{
		LeaseId: proto.String(id),
	})
	return err
}

func (s *rpcScheduler) done(id string) error {
	_, err := s.client.Done(context.Background(), &pb.DoneRequest{
		LeaseId: proto.String(id),
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
