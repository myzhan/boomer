package boomer

type client interface {
	connect()
	close()
	recvChannel() chan *message
	sendChannel() chan *message
	disconnectedChannel() chan bool
}
