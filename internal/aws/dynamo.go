package aws

import (
	"context"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type SessionRecord struct {
	TicketID   string
	EventID    string
	JTI        string
	AdmittedAt time.Time
}

// DynamoWriter writes admitted-session records to DynamoDB.
type DynamoWriter struct {
	client    *dynamodb.Client
	tableName string
}

func NewDynamoWriter(tableName string) (*DynamoWriter, error) {
	if os.Getenv("AWS_REGION") == "" {
		return nil, nil // ponytail: nil receiver is the local-dev no-op
	}
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}
	return &DynamoWriter{client: dynamodb.NewFromConfig(cfg), tableName: tableName}, nil
}

func (dw *DynamoWriter) Write(ctx context.Context, r SessionRecord) error {
	if dw == nil {
		return nil
	}
	_, err := dw.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(dw.tableName),
		Item: map[string]types.AttributeValue{
			"ticketId":   &types.AttributeValueMemberS{Value: r.TicketID},
			"eventId":    &types.AttributeValueMemberS{Value: r.EventID},
			"jti":        &types.AttributeValueMemberS{Value: r.JTI},
			"admittedAt": &types.AttributeValueMemberS{Value: r.AdmittedAt.UTC().Format(time.RFC3339)},
		},
	})
	return err
}
