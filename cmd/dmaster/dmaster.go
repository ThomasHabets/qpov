package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/sqs"

	"github.com/ThomasHabets/qpov/dist"
)

var (
	queueName = flag.String("queue", "qpov", "Name of SQS queue.")
)

func getAuth() aws.Auth {
	return aws.Auth{
		AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}
}

func main() {
	flag.Parse()
	conn := sqs.New(getAuth(), aws.USEast)
	q, err := conn.GetQueue(*queueName)
	if err != nil {
		log.Fatalf("Getting queue: %v", err)
	}
	order, err := json.Marshal(&dist.Order{
		Package:     "s3://qpov/balcony.rar",
		Dir:         "balcony",
		File:        "balcony.pov",
		Destination: "s3://qpov/balcony/",
		//Args:        []string{"+Q11", "+A0.3", "+R4", "+W3840", "+H2160"},
		Args: []string{"+W640", "+H480"},
	})
	if err != nil {
		log.Fatalf("Creating queue: %v", err)
	}
	if _, err := q.SendMessage(string(order)); err != nil {
		log.Fatalf("Failed to enqueue: %v", err)
	}
}
