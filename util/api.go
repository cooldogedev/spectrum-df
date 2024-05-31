package util

import (
	"fmt"
	"net"

	"github.com/cooldogedev/spectrum/api"
	"github.com/cooldogedev/spectrum/api/packet"
)

func NewClient(addr string, token string) (*api.Client, error) {
	c, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	client := api.NewClient(c, packet.NewPool())
	if err := client.WritePacket(&packet.ConnectionRequest{Token: token}); err != nil {
		_ = client.Close()
		return nil, err
	}

	connectionResponsePacket, err := client.ReadPacket()
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	connectionResponse, ok := connectionResponsePacket.(*packet.ConnectionResponse)
	if !ok {
		_ = client.Close()
		return nil, fmt.Errorf("expected connection response, got %d", connectionResponse.ID())
	}

	if connectionResponse.Response != packet.ResponseSuccess {
		_ = client.Close()
		return nil, fmt.Errorf("connection failed, code %d", connectionResponse.ID())
	}
	return client, nil
}
