package boomer

import (
	"github.com/ugorji/go/codec"
	"log"
)

var (
	mh codec.MsgpackHandle
)

type Message struct {
	Type   string                 `codec: "type"`
	Data   map[string]interface{} `codec: "data"`
	NodeId string                 `codec: "node_id"`
}

func (this *Message) Serialize() (out []byte) {
	mh.StructToArray = true
	enc := codec.NewEncoderBytes(&out, &mh)
	err := enc.Encode(this)
	if err != nil {
		log.Fatal("[msgpack] encode fail")
	}
	return
}

func NewMessageFromBytes(raw []byte) *Message {
	mh.StructToArray = true
	dec := codec.NewDecoderBytes(raw, &mh)
	var newMsg = &Message{}
	err := dec.Decode(newMsg)
	if err != nil {
		log.Fatal("[msgpack] decode fail")
	}
	return newMsg
}
