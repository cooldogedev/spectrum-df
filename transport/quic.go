package transport

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/qlog"
)

type QUIC struct {
	cert     tls.Certificate
	listener *quic.Listener

	incoming chan io.ReadWriteCloser
	closed   chan struct{}
}

func NewQUIC(cert tls.Certificate) *QUIC {
	return &QUIC{
		cert: cert,

		incoming: make(chan io.ReadWriteCloser),
		closed:   make(chan struct{}),
	}
}

func (q *QUIC) Listen(addr string) (err error) {
	listener, err := quic.ListenAddr(
		addr,
		&tls.Config{
			Certificates:       []tls.Certificate{q.cert},
			InsecureSkipVerify: true,
			NextProtos:         []string{"spectrum"},
		},
		&quic.Config{
			MaxIdleTimeout:                 time.Second * 10,
			InitialStreamReceiveWindow:     1024 * 1024 * 10,
			MaxStreamReceiveWindow:         1024 * 1024 * 10,
			InitialConnectionReceiveWindow: 1024 * 1024 * 10,
			MaxConnectionReceiveWindow:     1024 * 1024 * 10,
			KeepAlivePeriod:                time.Second * 5,
			InitialPacketSize:              1350,
			Tracer:                         qlog.DefaultConnectionTracer,
		},
	)
	if err != nil {
		return err
	}

	go func() {
		for {
			connection, err := listener.Accept(context.Background())
			if err != nil {
				return
			}
			go q.handle(connection)
		}
	}()
	q.listener = listener
	return
}

// Accept ...
func (q *QUIC) Accept() (io.ReadWriteCloser, error) {
	select {
	case <-q.closed:
		return nil, errors.New("closed listener")
	case c := <-q.incoming:
		return c, nil
	}
}

// Close ...
func (q *QUIC) Close() (err error) {
	select {
	case <-q.closed:
		return errors.New("already closed")
	default:
		close(q.closed)
		_ = q.listener.Close()
		return
	}
}

func (q *QUIC) handle(connection quic.Connection) {
	defer connection.CloseWithError(0, "")
	for {
		stream, err := connection.AcceptStream(context.Background())
		if err != nil {
			return
		}
		q.incoming <- stream
	}
}
