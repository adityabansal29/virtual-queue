package aws

import (
	"context"
	"encoding/json"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type AdmissionEvent struct {
	TicketID string `json:"ticketId"`
	EventID  string `json:"eventId"`
	JTI      string `json:"jti"`
}

// SQSEmitter publishes admission events to the FIFO queue.
type SQSEmitter struct {
	client   *sqs.Client
	queueURL string
}

func NewSQSEmitter(queueURL string) (*SQSEmitter, error) {
	if os.Getenv("AWS_REGION") == "" {
		return nil, nil // ponytail: nil receiver is the local-dev no-op
	}
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}
	return &SQSEmitter{client: sqs.NewFromConfig(cfg), queueURL: queueURL}, nil
}

func (se *SQSEmitter) Emit(ctx context.Context, e AdmissionEvent) error {
	if se == nil {
		return nil
	}
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = se.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:       aws.String(se.queueURL),
		MessageBody:    aws.String(string(body)),
		MessageGroupId: aws.String(e.EventID),
	})
	return err
}
