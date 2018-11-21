package boomer

import (
	"testing"
)

func TestEncodeAndDecode(t *testing.T) {

	data := make(map[string]interface{})
	data["a"] = 1
	data["b"] = "hello"
	msg := newMessage("test", data, "nodeID")

	encoded, err := msg.serialize()
	if err != nil {
		t.Error(err)
	}
	decoded, err := newMessageFromBytes(encoded)
	if err != nil {
		t.Error(err)
	}

	if msg.Type != decoded.Type {
		t.Error("message type mismatched.")
	}
	if msg.NodeID != decoded.NodeID {
		t.Error("message type mismatched.")
	}

	decodedA := decoded.Data["a"]
	decodedAInt := decodedA.(int64)
	decodedB := decoded.Data["b"]
	decodedBArray := decodedB.([]uint8)

	if msg.Data["a"] != int(decodedAInt) || msg.Data["b"] != string(decodedBArray) {
		t.Error("message data mismatched.", msg.Data, decoded.Data)
	}
}
