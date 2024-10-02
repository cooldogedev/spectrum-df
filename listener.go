package spectrum

import (
	"github.com/cooldogedev/spectrum-df/internal"
	tr "github.com/cooldogedev/spectrum-df/transport"
	"github.com/cooldogedev/spectrum-df/util"
	"github.com/df-mc/dragonfly/server/session"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type Listener struct {
	authentication util.Authentication
	transport      tr.Transport
}

func NewListener(addr string, authentication util.Authentication, transport tr.Transport) (*Listener, error) {
	if transport == nil {
		transport = tr.NewSpectral()
	}

	if err := transport.Listen(addr); err != nil {
		return nil, err
	}
	return &Listener{
		authentication: authentication,
		transport:      transport,
	}, nil
}

// Accept ...
func (l *Listener) Accept() (session.Conn, error) {
	c, err := l.transport.Accept()
	if err != nil {
		return nil, err
	}

	var authenticator internal.Authenticator
	if l.authentication != nil {
		authenticator = l.authentication.Authenticate
	}
	return internal.NewConn(c, authenticator, packet.NewClientPool())
}

// Disconnect ...
func (l *Listener) Disconnect(conn session.Conn, _ string) error {
	return conn.Close()
}

// Close ...
func (l *Listener) Close() error {
	return l.transport.Close()
}
