package spectrum

import (
	"net"

	packet2 "github.com/cooldogedev/spectrum/server/packet"
	"github.com/df-mc/dragonfly/server/session"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type Authentication interface {
	Authenticate(identityData login.IdentityData, token string) bool
}

type Listener struct {
	l    net.Listener
	auth Authentication
}

func NewListener(addr string, auth Authentication) (Listener, error) {
	netListener, err := net.Listen("tcp", addr)
	return Listener{
		l:    netListener,
		auth: auth,
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
	return newConn(c, l.auth, packet.NewClientPool())
}

// Disconnect ...
func (l Listener) Disconnect(conn session.Conn, _ string) error {
	return conn.Close()
}

// Close ...
func (l Listener) Close() error {
	return l.l.Close()
}

var packetDecode = map[uint32]bool{}

func RegisterPacketDecode(id uint32, value bool) {
	packetDecode[id] = value
}

func init() {
	RegisterPacketDecode(packet2.IDLatency, true)
	RegisterPacketDecode(packet2.IDTransfer, true)

	RegisterPacketDecode(packet.IDAddActor, true)
	RegisterPacketDecode(packet.IDAddItemActor, true)
	RegisterPacketDecode(packet.IDAddPainting, true)
	RegisterPacketDecode(packet.IDAddPlayer, true)

	RegisterPacketDecode(packet.IDBossEvent, true)

	RegisterPacketDecode(packet.IDMobEffect, true)

	RegisterPacketDecode(packet.IDPlayerList, true)

	RegisterPacketDecode(packet.IDRemoveActor, true)
	RegisterPacketDecode(packet.IDRemoveObjective, true)

	RegisterPacketDecode(packet.IDSetDisplayObjective, true)
}
