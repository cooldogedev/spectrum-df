package spectrum

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/cooldogedev/spectrum-df/internal"
	"github.com/cooldogedev/spectrum/api/packet"
	"github.com/cooldogedev/spectrum/protocol"
)

type Client struct {
	conn   net.Conn
	reader *protocol.Reader
	writer *protocol.Writer
	pool   packet.Pool
}

func NewClient(addr string, token string) (*Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	c := &Client{
		conn:   conn,
		reader: protocol.NewReader(conn),
		writer: protocol.NewWriter(conn),
		pool:   packet.NewPool(),
	}

	if err := c.WritePacket(&packet.ConnectionRequest{Token: token}); err != nil {
		_ = c.Close()
		return nil, err
	}

	connectionResponsePacket, err := c.ReadPacket()
	if err != nil {
		_ = c.Close()
		return nil, err
	}

	connectionResponse, ok := connectionResponsePacket.(*packet.ConnectionResponse)
	if !ok {
		_ = c.Close()
		return nil, fmt.Errorf("expected connection response, got %d", connectionResponse.ID())
	}

	if connectionResponse.Response != packet.ResponseSuccess {
		_ = c.Close()
		return nil, fmt.Errorf("connection failed, code %d", connectionResponse.ID())
	}
	return c, nil
}

func (c *Client) ReadPacket() (pk packet.Packet, err error) {
	payload, err := c.reader.ReadPacket()
	if err != nil {
		return nil, err
	}

	buf := internal.BufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		internal.BufferPool.Put(buf)

		if r := recover(); r != nil {
			err = fmt.Errorf("panic while decoding packet: %v", r)
		}
	}()

	packetID := binary.LittleEndian.Uint32(payload[:4])
	factory, ok := c.pool[packetID]
	if !ok {
		return nil, fmt.Errorf("unknown packet ID: %v", packetID)
	}

	buf.Write(payload[4:])
	pk = factory()
	pk.Decode(buf)
	return
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
	return c.conn.Close()
}
