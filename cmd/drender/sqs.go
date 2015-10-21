package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/sqs"
)

func getAuth() aws.Auth {
	return aws.Auth{
		AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}
}

type sqsScheduler struct {
	q *sqs.Queue
}

func (*sqsScheduler) get() (string, string, error) {
	/*	r, err := s.q.ReceiveMessage(1)
		if err != nil {
			return "", "", err
		}
		if len(r.Messages) == 0 {
			return "", "", nil
		}
		return "", r.Messages[0].Body
	*/
	return "", "", fmt.Errorf("not implemented")
}

func (s *sqsScheduler) renew(id string, dur time.Duration) error {
	//_, err := s.q.ChangeMessageVisibility(id, int(dur.Seconds())
	//return err
	return fmt.Errorf("not implemented")
}

func (s *sqsScheduler) done(id string, img, stdout, stderr []byte, j string) error {
	//_, err := s.q.DeleteMessage(&m)
	return fmt.Errorf("not implemented")
}

func newSQSScheduler() scheduler {
	conn := sqs.New(getAuth(), aws.USEast)
	q, err := conn.GetQueue(*queueName)
	if err != nil {
		log.Fatalf("Getting queue: %v", err)
	}
	return &sqsScheduler{q: q}
}
