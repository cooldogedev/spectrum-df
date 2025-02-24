package transport

import (
	"github.com/xtaci/kcp-go"
	"io"
	"net"
)

type KCP struct {
	listener net.Listener
}

func NewKCP() *KCP {
	return &KCP{}
}

// Listen ...
func (k *KCP) Listen(addr string) (err error) {
	listener, err := kcp.ListenWithOptions(addr, nil, 10, 3)
	if err != nil {
		return err
	}
	k.listener = listener
	return
}

// Accept ...
func (k *KCP) Accept() (io.ReadWriteCloser, error) {
	c, err := k.listener.Accept()
	if err != nil {
		return nil, err
	}

	return c, nil
}

// Close ...
func (k *KCP) Close() error {
	return k.listener.Close()
}
