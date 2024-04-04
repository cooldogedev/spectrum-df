package spectrum

import (
	"github.com/df-mc/dragonfly/server/session"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"net"
)

type Listener struct {
	l net.Listener
}

func NewListener(addr string) (Listener, error) {
	netListener, err := net.Listen("tcp", addr)
	return Listener{
		l: netListener,
	}, err
}

// Accept ...
func (l Listener) Accept() (session.Conn, error) {
	c, err := l.l.Accept()
	if err != nil {
		return nil, err
	}

	if tcpConn, ok := c.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
		_ = tcpConn.SetLinger(0)
		_ = tcpConn.SetReadBuffer(1024 * 1024 * 8)
		_ = tcpConn.SetWriteBuffer(1024 * 1024 * 8)
	}
	return newConn(c, packet.NewClientPool())
}

// Disconnect ...
func (l Listener) Disconnect(conn session.Conn, _ string) error {
	return conn.Close()
}

// Close ...
func (l Listener) Close() error {
	return l.l.Close()
}
