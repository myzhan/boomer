package boomer

import (
	"github.com/ugorji/go/codec"
	"log"
)

var (
	mh codec.MsgpackHandle
)

type message struct {
	Type   string                 `codec: "type"`
	Data   map[string]interface{} `codec: "data"`
	NodeId string                 `codec: "node_id"`
}

func newMessage(t string, data map[string]interface{}, nodeId string) (msg *message) {
	return &message{
		Type:   t,
		Data:   data,
		NodeId: nodeId,
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
