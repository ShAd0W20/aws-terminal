package sqs

import "time"

type Queue struct {
	Name              string
	URL               string
	ARN               string
	AvailableMessages int64
	InFlightMessages  int64
}

type Message struct {
	ID            string
	Body          string
	ReceiptHandle string
	SentAt        time.Time
}
