package spectrum

import (
	tr "github.com/cooldogedev/spectrum-df/transport"
	"github.com/df-mc/dragonfly/server/session"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type Listener struct {
	transport tr.Transport
}

func NewListener(addr string, transport tr.Transport) (*Listener, error) {
	if transport == nil {
		transport = tr.NewSpectral()
	}

	if err := transport.Listen(addr); err != nil {
		return nil, err
	}
	return &Listener{transport: transport}, nil
}

// Accept ...
func (l *Listener) Accept() (session.Conn, error) {
	c, err := l.transport.Accept()
	if err != nil {
		return nil, err
	}
	return newConn(c, packet.NewClientPool())
}

// Disconnect ...
func (l *Listener) Disconnect(conn session.Conn, reason string) error {
	_ = conn.WritePacket(&packet.Disconnect{
		HideDisconnectionScreen: reason == "",
		Message:                 reason,
	})
	return conn.Close()
}

// Close ...
func (l *Listener) Close() error {
	return l.transport.Close()
}
