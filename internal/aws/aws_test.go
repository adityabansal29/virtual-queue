package aws

import (
	"context"
	"testing"
)

func TestDynamoWriterNilGuard(t *testing.T) {
	if err := (*DynamoWriter)(nil).Write(context.Background(), SessionRecord{}); err != nil {
		t.Fatal(err)
	}
}

func TestSQSEmitterNilGuard(t *testing.T) {
	if err := (*SQSEmitter)(nil).Emit(context.Background(), AdmissionEvent{}); err != nil {
		t.Fatal(err)
	}
}
