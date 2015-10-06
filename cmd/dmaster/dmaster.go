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
	queueName = flag.String("queue", "", "Name of SQS queue.")
	pkg       = flag.String("package", "", "S3 path to rar file containing all resources.")
	dir       = flag.String("dir", "", "Directory in package to use as CWD.")
	file      = flag.String("file", "", "POV file to render.")
	dst       = flag.String("destination", "", "S3 directory to store results in.")
)

func getAuth() aws.Auth {
	return aws.Auth{
		AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}
}

func main() {
	flag.Parse()

	if *queueName == "" {
		log.Fatalf("Must supply -queue")
	}
	if *pkg == "" {
		log.Fatalf("Must supply -package")
	}
	if *file == "" {
		log.Fatalf("Must supply -file")
	}
	if *dst == "" {
		log.Fatalf("Must supply -destination")
	}

	conn := sqs.New(getAuth(), aws.USEast)
	q, err := conn.GetQueue(*queueName)
	if err != nil {
		log.Fatalf("Getting queue: %v", err)
	}

	order, err := json.Marshal(&dist.Order{
		Package:     *pkg,
		Dir:         *dir,
		File:        *file,
		Destination: *dst,
		Args:        flag.Args(),
		//Args:        []string{"+Q11", "+A0.3", "+R4", "+W3840", "+H2160"},
		//Args: []string{"+W320", "+H240"},
	})
	if err != nil {
		log.Fatalf("JSON-marshaling order: %v", err)
	}
	if _, err := q.SendMessage(string(order)); err != nil {
		log.Fatalf("Failed to enqueue: %v", err)
	}
}
