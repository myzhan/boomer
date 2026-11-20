//go:build !goczmq
// +build !goczmq

package boomer

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/myzhan/gomq/zmtp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test gomq client", func() {

	It("test connect with error", func() {
		client := newClient("mock:0.0.0.0", 1234, "testing")
		mockGomqDealerInstance.SetConnectError(errors.New("connect error"))
		defer mockGomqDealerInstance.SetConnectError(nil)
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
		Eventually(mockGomqDealerInstance.SendChannel()).Should(Receive(Equal(pingMessageInBytes)))

		serverMessage := &zmtp.Message{
			MessageType: zmtp.UserMessage,
			Body:        [][]byte{pongMessageInBytes},
		}
		mockGomqDealerInstance.RecvChannel() <- serverMessage
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
		Eventually(mockGomqDealerInstance.SendChannel()).Should(Receive(Equal(pingMessageInBytes)))

		serverZmtpMessage := &zmtp.Message{
			MessageType: zmtp.UserMessage,
			Body:        [][]byte{pongMessageInBytes},
		}
		mockGomqDealerInstance.RecvChannel() <- serverZmtpMessage
		Eventually(client.recvChannel()).Should(Receive(Equal(pongMessage)))
	})
})

func drainMockGomqSendChannel() {
	for {
		select {
		case <-mockGomqDealerInstance.SendChannel():
		default:
			return
		}
	}
}

func drainMockGomqRecvChannel() {
	for {
		select {
		case <-mockGomqDealerInstance.RecvChannel():
		default:
			return
		}
	}
}

func waitForMockGomqSend(t *testing.T, want []byte, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case got := <-mockGomqDealerInstance.SendChannel():
			if bytes.Equal(got, want) {
				return
			}
		case <-deadline:
			t.Fatalf("timeout waiting for %s", string(want))
		}
	}
}

func newMockClient(t *testing.T, identity string) *gomqSocketClient {
	t.Helper()
	drainMockGomqSendChannel()
	drainMockGomqRecvChannel()

	c := newClient("mock:0.0.0.0", 1234, identity)
	if err := c.connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	return c
}

func TestGomqClientCloseIsIdempotent(t *testing.T) {
	c := newMockClient(t, "regression-close")
	c.close()
	// Closing twice used to panic with "close of closed channel".
	c.close()
}

// Sending the quit notification used to block the send goroutine when nobody was
// waiting on disconnectedChannel, which stopped every following message.
func TestGomqClientQuitNotificationDoesNotBlock(t *testing.T) {
	c := newMockClient(t, "regression-quit")
	defer c.close()

	// Nobody consumes disconnectedChannel, fill up its only slot.
	c.disconnectedFromMaster <- true

	c.sendChannel() <- newGenericMessage("quit", nil, "regression-quit")
	ping := newGenericMessage("ping", nil, "regression-quit")
	pingBytes, err := ping.serialize()
	if err != nil {
		t.Fatalf("failed to serialize message: %v", err)
	}
	c.sendChannel() <- ping

	// If the send goroutine got stuck on the quit notification, ping is never sent.
	waitForMockGomqSend(t, pingBytes, 5*time.Second)
}

func TestGomqClientRecvStopsAfterClose(t *testing.T) {
	c := newMockClient(t, "regression-recv")
	c.close()

	// Give the recv goroutine a chance to observe the closed shutdown channel.
	time.Sleep(100 * time.Millisecond)

	ping := newGenericMessage("ping", nil, "regression-recv")
	pingBytes, err := ping.serialize()
	if err != nil {
		t.Fatalf("failed to serialize message: %v", err)
	}
	mockGomqDealerInstance.RecvChannel() <- &zmtp.Message{
		MessageType: zmtp.UserMessage,
		Body:        [][]byte{pingBytes},
	}

	time.Sleep(100 * time.Millisecond)
	select {
	case msg := <-c.fromMaster:
		t.Fatalf("recv goroutine is still running after close, got %T", msg)
	default:
	}
}
