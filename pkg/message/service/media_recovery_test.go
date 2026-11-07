package message_service

import (
	"encoding/json"
	"testing"
)

func TestDownloadMediaRequestPreservesNativeMessageKey(t *testing.T) {
	raw := []byte(`{"key":{"id":"SYNTHETIC-MEDIA","remoteJid":"15550000001@s.whatsapp.net","fromMe":false},"message":{"audioMessage":{"directPath":"/v/synthetic.enc?oe=1"}}}`)
	var request DownloadMediaStruct
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip map[string]json.RawMessage
	if err = json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip["key"] == nil {
		t.Fatal("download request discarded the native message key required for retry correlation")
	}
}
