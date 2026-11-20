package boomer

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/ugorji/go/codec"
)

var _ = Describe("Test message", func() {

	It("test encode and decode", func() {
		data := make(map[string]interface{})
		data["a"] = 1
		data["b"] = "hello"
		msg := newGenericMessage("test", data, "nodeID")

		encoded, err := msg.serialize()
		Expect(err).NotTo(HaveOccurred())

		decoded, err := newGenericMessageFromBytes(encoded)
		Expect(err).NotTo(HaveOccurred())

		Expect(msg.Type).To(Equal(decoded.Type))
		Expect(msg.NodeID).To(Equal(decoded.NodeID))

		decodedA := decoded.Data["a"]
		decodedAInt := decodedA.(int64)
		decodedB := decoded.Data["b"]
		decodedBArray := decodedB.([]uint8)

		Expect(msg.Data["a"]).To(BeEquivalentTo(decodedAInt))
		Expect(msg.Data["b"]).To(BeEquivalentTo(decodedBArray))
	})
})

func decodedString(t *testing.T, value interface{}) string {
	t.Helper()
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		t.Fatalf("expected a string, got %T", value)
		return ""
	}
}

// locust decodes every message as a Message namedtuple, which is encoded as a
// 3 elements msgpack array: [type, data, node_id].
func TestMessageWireFormatMatchesLocust(t *testing.T) {
	msg := newGenericMessage("stats", map[string]interface{}{"user_count": int32(1)}, "node-1")
	raw, err := msg.serialize()
	if err != nil {
		t.Fatalf("failed to serialize message: %v", err)
	}

	var decoded []interface{}
	dec := codec.NewDecoderBytes(raw, &mh)
	if err := dec.Decode(&decoded); err != nil {
		t.Fatalf("failed to decode message: %v", err)
	}
	if len(decoded) != 3 {
		t.Fatalf("locust Message is a 3 fields namedtuple, got %d fields", len(decoded))
	}
	if got := decodedString(t, decoded[0]); got != "stats" {
		t.Fatalf("expected type %q, got %q", "stats", got)
	}
	if got := decodedString(t, decoded[2]); got != "node-1" {
		t.Fatalf("expected node_id %q, got %q", "node-1", got)
	}

	data, ok := decoded[1].(map[interface{}]interface{})
	if !ok {
		t.Fatalf("expected data to be a map, got %T", decoded[1])
	}
	if _, ok := data["user_count"]; !ok {
		t.Fatal("expected user_count in data")
	}
}

// locust master rejects a client_ready message without data, and accepts -1 to
// bypass the version check.
func TestClientReadyMessageMatchesLocust(t *testing.T) {
	msg := newClientReadyMessage("client_ready", -1, "node-1")
	raw, err := msg.serialize()
	if err != nil {
		t.Fatalf("failed to serialize message: %v", err)
	}

	var decoded []interface{}
	dec := codec.NewDecoderBytes(raw, &mh)
	if err := dec.Decode(&decoded); err != nil {
		t.Fatalf("failed to decode message: %v", err)
	}
	if len(decoded) != 3 {
		t.Fatalf("locust Message is a 3 fields namedtuple, got %d fields", len(decoded))
	}
	if got := decodedString(t, decoded[0]); got != "client_ready" {
		t.Fatalf("expected type %q, got %q", "client_ready", got)
	}
	if version, ok := castToInt64(decoded[1]); !ok || version != -1 {
		t.Fatalf("expected the version to be -1 to bypass the version check, got %v", decoded[1])
	}
	if got := decodedString(t, decoded[2]); got != "node-1" {
		t.Fatalf("expected node_id %q, got %q", "node-1", got)
	}
}
