package boomer

import (
	"github.com/ugorji/go/codec"
)

var (
	mh codec.MsgpackHandle
)

type message interface {
	serialize() (out []byte, err error)
}

type genericMessage struct {
	Type   string                 `codec:"type"`
	Data   map[string]interface{} `codec:"data"`
	NodeID string                 `codec:"node_id"`
}

func newGenericMessage(t string, data map[string]interface{}, nodeID string) (msg *genericMessage) {
	return &genericMessage{
		Type:   t,
		Data:   data,
		NodeID: nodeID,
	}
}

func (m *genericMessage) serialize() (out []byte, err error) {
	mh.StructToArray = true
	enc := codec.NewEncoderBytes(&out, &mh)
	err = enc.Encode(m)
	return out, err
}

func newGenericMessageFromBytes(raw []byte) (newMsg *genericMessage, err error) {
	mh.StructToArray = true
	dec := codec.NewDecoderBytes(raw, &mh)
	newMsg = &genericMessage{}
	err = dec.Decode(newMsg)
	return newMsg, err
}

type clientReadyMessage struct {
	Type   string `codec:"type"`
	Data   int    `codec:"data"`
	NodeID string `codec:"node_id"`
}

func newClientReadyMessage(t string, data int, nodeID string) (msg *clientReadyMessage) {
	return &clientReadyMessage{
		Type:   t,
		Data:   data,
		NodeID: nodeID,
	}
}

func (m *clientReadyMessage) serialize() (out []byte, err error) {
	mh.StructToArray = true
	enc := codec.NewEncoderBytes(&out, &mh)
	err = enc.Encode(m)
	return out, err
}

func newClientReadyMessageFromBytes(raw []byte) (newMsg *clientReadyMessage, err error) {
	mh.StructToArray = true
	dec := codec.NewDecoderBytes(raw, &mh)
	newMsg = &clientReadyMessage{}
	err = dec.Decode(newMsg)
	return newMsg, err
}
