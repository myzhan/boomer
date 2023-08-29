//go:build !goczmq
// +build !goczmq

package boomer

import (
	"errors"

	"github.com/myzhan/gomq/zmtp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test gomq client", func() {

	It("test connect with error", func() {
		client := newClient("mock:0.0.0.0", 1234, "testing")
		MockGomqDealerInstance.SetConnectError(errors.New("connect error"))
		defer MockGomqDealerInstance.SetConnectError(nil)
		err := client.connect()
		Expect(err).To(HaveOccurred())
	})

	It("test server send generic message", func() {
		client := newClient("mock:0.0.0.0", 1234, "testing")
		client.connect()
		defer client.close()

		pingMessage := newGenericMessage("ping", nil, "testing")
		pingMessageInBytes, _ := pingMessage.serialize()
		pongMessage := newGenericMessage("pong", nil, "testing")
		pongMessageInBytes, _ := pongMessage.serialize()

		client.sendChannel() <- pingMessage
		Eventually(MockGomqDealerInstance.SendChannel()).Should(Receive(Equal(pingMessageInBytes)))

		serverMessage := &zmtp.Message{
			MessageType: zmtp.UserMessage,
			Body:        [][]byte{pongMessageInBytes},
		}
		MockGomqDealerInstance.RecvChannel() <- serverMessage
		Eventually(client.recvChannel()).Should(Receive(Equal(pongMessage)))
	})

	It("test server send custom message", func() {
		client := newClient("mock:0.0.0.0", 1234, "testing")
		client.connect()
		defer client.close()

		pingMessage := newGenericMessage("ping", nil, "testing")
		pingMessageInBytes, _ := pingMessage.serialize()
		pongMessage := newCustomMessage("pong", int64(123), "testing")
		pongMessageInBytes, _ := pongMessage.serialize()

		client.sendChannel() <- pingMessage
		Eventually(MockGomqDealerInstance.SendChannel()).Should(Receive(Equal(pingMessageInBytes)))

		serverZmtpMessage := &zmtp.Message{
			MessageType: zmtp.UserMessage,
			Body:        [][]byte{pongMessageInBytes},
		}
		MockGomqDealerInstance.RecvChannel() <- serverZmtpMessage
		Eventually(client.recvChannel()).Should(Receive(Equal(pongMessage)))
	})
})
