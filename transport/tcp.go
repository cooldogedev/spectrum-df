package transport

import (
	"io"
	"net"
)

type TCP struct {
	listener net.Listener
}

func NewTCP() *TCP {
	return &TCP{}
}

// Listen ...
func (t *TCP) Listen(addr string) (err error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	t.listener = listener
	return
}

// Accept ...
func (t *TCP) Accept() (io.ReadWriteCloser, error) {
	c, err := t.listener.Accept()
	if err != nil {
		return nil, err
	}

	if tcpConn, ok := c.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
		_ = tcpConn.SetLinger(0)
		_ = tcpConn.SetReadBuffer(1024 * 1024 * 8)
		_ = tcpConn.SetWriteBuffer(1024 * 1024 * 8)
	}
	return c, nil
}

// Close ...
func (t *TCP) Close() error {
	return t.listener.Close()
}
