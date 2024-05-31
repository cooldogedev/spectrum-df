package transport

import "io"

type Transport interface {
	Listen(string) error
	Accept() (io.ReadWriteCloser, error)
	Close() error
}
