package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/ThomasHabets/go-uuid/uuid"
	"github.com/golang/protobuf/proto"
	_ "github.com/lib/pq"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

var (
	dbConnect        = flag.String("db", "", "")
	defaultLeaseTime = 60 * time.Second
	db               *sql.DB
	addr             = flag.String("port", ":9999", "Addr to listen to.")
)

type server struct{}

func (s *server) Get(ctx context.Context, in *pb.GetRequest) (*pb.GetReply, error) {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Database error: %v", err)
		return nil, fmt.Errorf("database error")
	}

	defer tx.Rollback()
	row := tx.QueryRow(`
SELECT orders.order_id, orders.definition
FROM orders
LEFT OUTER JOIN leases ON orders.order_id=leases.order_id
WHERE leases.lease_id IS NULL
LIMIT 1`)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("nothing to do")
	} else if err != nil {
		log.Printf("Database error QueryRow: %v", err)
		return nil, fmt.Errorf("database error")
	}

	var orderID, def string
	if err := row.Scan(&orderID, &def); err == sql.ErrNoRows {
		return nil, fmt.Errorf("nothing to do")
	} else if err != nil {
		log.Printf("Database error scanning: %v", err)
		return nil, fmt.Errorf("database error")
	}

	lease := uuid.New()
	if _, err := tx.Exec(`INSERT INTO leases(lease_id, done, order_id, user_id, created, updated, expires)
VALUES($1, false, $2, $3, NOW(), NOW(), $4)`, lease, orderID, 1, time.Now().Add(defaultLeaseTime)); err != nil {
		log.Printf("Database error inserting lease: %v", err)
		return nil, fmt.Errorf("database error")
	}
	if err := tx.Commit(); err != nil {
		log.Printf("Database error committing: %v", err)
		return nil, fmt.Errorf("database error")
	}
	return &pb.GetReply{
		LeaseId:         proto.String(lease),
		OrderDefinition: proto.String(def),
	}, nil
}

func (s *server) Done(ctx context.Context, in *pb.DoneRequest) (*pb.DoneReply, error) {
	_, err := db.Exec(`UPDATE leases SET done=TRUE WHERE lease_id=$1`, in.GetLeaseId())
	if err != nil {
		return nil, err
	}
	return &pb.DoneReply{}, nil
}

func (s *server) Add(ctx context.Context, in *pb.AddRequest) (*pb.AddReply, error) {
	id := uuid.New()
	_, err := db.Exec(`INSERT INTO orders(order_id, owner, definition) VALUES($1,$2,$3)`, id, 1, in.GetOrderDefinition())
	if err != nil {
		return nil, err
	}
	return &pb.AddReply{
		OrderId: proto.String(id),
	}, nil
}

func (s *server) Renew(ctx context.Context, in *pb.RenewRequest) (*pb.RenewReply, error) {
	_, err := db.Exec(`UPDATE leases SET updated=NOW(), expires=$1 WHERE lease_id=$2`, time.Now().Add(defaultLeaseTime), in.GetLeaseId())
	if err != nil {
		return nil, err
	}
	return &pb.RenewReply{}, nil
}

func main() {
	flag.Parse()
	var err error
	db, err = sql.Open("postgres", *dbConnect)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterSchedulerServer(s, &server{})
	log.Printf("Running...")
	log.Fatal(s.Serve(lis))
}
