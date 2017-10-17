package zmtp

import "errors"

// Socket is a ZMTP socket
type Socket interface {
	Type() SocketType
	IsSocketTypeCompatible(socketType SocketType) bool
	IsCommandTypeValid(name string) bool
}

// NewSocket returns a new ZMTP socket
func NewSocket(socketType SocketType) (Socket, error) {
	switch socketType {
	case ClientSocketType:
		return clientSocket{}, nil
	case ServerSocketType:
		return serverSocket{}, nil
	case PullSocketType:
		return pullSocket{}, nil
	case PushSocketType:
		return pushSocket{}, nil
	default:
		return nil, errors.New("Invalid socket type")
	}
}

type clientSocket struct{}

// Type returns the Socket's type
func (clientSocket) Type() SocketType {
	return ClientSocketType
}

// IsSocketTypeCompatible checks if the socket is compatible with
// another socket type.
func (clientSocket) IsSocketTypeCompatible(socketType SocketType) bool {
	return socketType == ServerSocketType
}

// IsCommandTypeValid returns if a command is valid for this socket.
func (clientSocket) IsCommandTypeValid(name string) bool {
	return false
}

type serverSocket struct{}

// Type returns the Socket's type
func (serverSocket) Type() SocketType {
	return ServerSocketType
}

// IsSocketTypeCompatible checks if the socket is compatible with
// another socket type.
func (serverSocket) IsSocketTypeCompatible(socketType SocketType) bool {
	return socketType == ClientSocketType
}

// IsCommandTypeValid returns if a command is valid for this socket.
func (serverSocket) IsCommandTypeValid(name string) bool {
	return false
}

type pullSocket struct{}

// Type returns the Socket's type
func (pullSocket) Type() SocketType {
	return PullSocketType
}

// IsSocketTypeCompatible checks if the socket is compatible with
// another socket type.
func (pullSocket) IsSocketTypeCompatible(socketType SocketType) bool {
	return socketType == PushSocketType
}

// IsCommandTypeValid returns if a command is valid for this socket.
func (pullSocket) IsCommandTypeValid(name string) bool {
	// FIXME(sbinet)
	return false
}

type pushSocket struct{}

// Type returns the Socket's type
func (pushSocket) Type() SocketType {
	return PushSocketType
}

// IsSocketTypeCompatible checks if the socket is compatible with
// another socket type.
func (pushSocket) IsSocketTypeCompatible(socketType SocketType) bool {
	return socketType == PullSocketType
}

// IsCommandTypeValid returns if a command is valid for this socket.
func (pushSocket) IsCommandTypeValid(name string) bool {
	// FIXME(sbinet)
	return false
}
