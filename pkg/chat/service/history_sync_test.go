package chat_service

import (
	"context"
	"errors"
	"testing"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
)

type historySyncSenderStub struct {
	message *waE2E.Message
	calls   int
	err     error
}

func (s *historySyncSenderStub) SendPeerMessage(_ context.Context, message *waE2E.Message) (whatsmeow.SendResponse, error) {
	s.calls++
	s.message = message
	return whatsmeow.SendResponse{}, s.err
}

func TestSendHistorySyncRequestTargetsOwnDevicesAndPreservesPayload(t *testing.T) {
	payload := &waE2E.Message{}
	sender := &historySyncSenderStub{}

	response, err := sendHistorySyncRequest(context.Background(), sender, payload)
	if err != nil {
		t.Fatal(err)
	}
	if sender.calls != 1 || sender.message != payload {
		t.Fatalf("peer sends = %d, payload preserved = %t", sender.calls, sender.message == payload)
	}
	if response == nil {
		t.Fatal("successful peer send returned no response")
	}

	sender.err = errors.New("synthetic send failure")
	sender.calls = 0
	response, err = sendHistorySyncRequest(context.Background(), sender, payload)
	if err != sender.err || response != nil {
		t.Fatalf("failed peer send = (%v, %v), want (nil, %v)", response, err, sender.err)
	}
	if sender.calls != 1 {
		t.Fatalf("peer sends = %d, want 1 on failure", sender.calls)
	}
}
