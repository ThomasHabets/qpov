package main

import (
	"log"
	"os"

	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/sqs"
)

type sqsScheduler struct {
	q *sqs.Queue
}

func (s *sqsScheduler) add(order, batchID string) error {
	_, err := s.q.SendMessage(string(order))
	return err
}

func (s *sqsScheduler) close() {
}

func getAuth() aws.Auth {
	return aws.Auth{
		AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}
}

func newSQS() scheduler {
	conn := sqs.New(getAuth(), aws.USEast)
	q, err := conn.GetQueue(*queueName)
	if err != nil {
		log.Fatalf("Getting queue: %v", err)
	}
	return &sqsScheduler{q: q}
}
