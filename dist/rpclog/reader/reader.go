package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang/protobuf/proto"

	qpovpb "github.com/ThomasHabets/qpov/dist/qpov"
	"github.com/ThomasHabets/qpov/dist/rpclog"
	pb "github.com/ThomasHabets/qpov/dist/rpclog/proto"
)

var (
	details = flag.Bool("details", false, "Show request and reply proto.")
)

func anyText(msg *pb.Any) string {
	m := map[string]proto.Message{
		"github.com/ThomasHabets/qpov/dist/qpov/RenewRequest": &qpovpb.RenewRequest{},
		"github.com/ThomasHabets/qpov/dist/qpov/RenewReply":   &qpovpb.RenewReply{},
		"github.com/ThomasHabets/qpov/dist/qpov/GetRequest":   &qpovpb.GetRequest{},
		"github.com/ThomasHabets/qpov/dist/qpov/GetReply":     &qpovpb.GetReply{},
		"github.com/ThomasHabets/qpov/dist/qpov/StatsRequest": &qpovpb.StatsRequest{},
		"github.com/ThomasHabets/qpov/dist/qpov/StatsReply":   &qpovpb.StatsReply{},
		"github.com/ThomasHabets/qpov/dist/qpov/DoneRequest":  &qpovpb.DoneRequest{},
		"github.com/ThomasHabets/qpov/dist/qpov/DoneReply":    &qpovpb.DoneReply{},
	}
	d, ok := m[msg.TypeUrl]
	if !ok {
		return fmt.Sprintf("%x", msg.Value)
	}
	if err := proto.Unmarshal(msg.Value, d); err != nil {
		return fmt.Sprintf("%x", msg.Value)
	}
	return d.String()
}

func main() {
	flag.Parse()
	var r *rpclog.Reader
	{
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			log.Fatalf("Failed to open %q: %v", flag.Arg(0), err)
		}
		r = rpclog.NewReader(f)
	}
	{
		const f = "%10v %10v %20s %8s %20s %15s\n"
		fmt.Printf(f, "Time", "RT (ns)", "Addr", "CN", "Method", "Error")
		for s := range r.Stream() {
			fmt.Printf(f, s.StartNs/1000000000, s.EndNs-s.StartNs, s.Peer.Address, s.Peer.CommonName, s.Method, s.Error)
			if *details {
				fmt.Printf("  Request: %s\n  Response: %s\n", anyText(s.Request), anyText(s.Response))
			}
		}
	}
}
