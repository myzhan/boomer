package boomer

import (
	"log"

	"github.com/ugorji/go/codec"
)

var (
	mh codec.MsgpackHandle
)

type message struct {
	Type   string                 `codec:"type"`
	Data   map[string]interface{} `codec:"data"`
	NodeID string                 `codec:"node_id"`
}

func newMessage(t string, data map[string]interface{}, nodeID string) (msg *message) {
	return &message{
		Type:   t,
		Data:   data,
		NodeID: nodeID,
	}
}

func (m *message) serialize() (out []byte) {
	mh.StructToArray = true
	enc := codec.NewEncoderBytes(&out, &mh)
	err := enc.Encode(m)
	if err != nil {
		log.Fatal("[msgpack] encode fail")
	}
	return
}

func newMessageFromBytes(raw []byte) *message {
	mh.StructToArray = true
	dec := codec.NewDecoderBytes(raw, &mh)
	var newMsg = &message{}
	err := dec.Decode(newMsg)
	if err != nil {
		log.Fatal("[msgpack] decode fail")
	}
	return newMsg
}
