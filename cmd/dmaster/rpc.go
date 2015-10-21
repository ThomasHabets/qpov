package main

import (
	"log"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pb "github.com/ThomasHabets/qpov/dist/qpov"
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
		OrderDefinition: proto.String(order),
	})
	return err
}

func newRPCScheduler() scheduler {
	conn, err := grpc.Dial(*schedAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	return &rpcScheduler{
		client: pb.NewSchedulerClient(conn),
	}
}
