package spectrum

import (
	"bytes"
	"encoding/binary"
	"github.com/cooldogedev/spectrum-df/internal"
	"github.com/cooldogedev/spectrum/api/packet"
	proto "github.com/cooldogedev/spectrum/protocol"
	"net"
)

type Client struct {
	c      net.Conn
	writer *proto.Writer
}

func NewClient(addr string) (*Client, error) {
	c, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &Client{
		c:      c,
		writer: proto.NewWriter(c),
	}, nil
}

func (c *Client) WritePacket(pk packet.Packet) error {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		internal.BufferPool.Put(buf)
	}()

	if err := binary.Write(buf, binary.LittleEndian, pk.ID()); err != nil {
		return err
	}
	pk.Encode(buf)
	return c.writer.Write(buf.Bytes())
}

func (c *Client) Close() error {
	return c.c.Close()
}
