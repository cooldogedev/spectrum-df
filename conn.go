package spectrum

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/cooldogedev/spectrum-df/internal"
	proto "github.com/cooldogedev/spectrum/protocol"
	packet2 "github.com/cooldogedev/spectrum/server/packet"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	packetDecodeNeeded    = 0x00
	packetDecodeNotNeeded = 0x01
)

var packetsDecode = map[uint32]bool{
	packet2.IDLatency:  true,
	packet2.IDTransfer: true,

	packet.IDAddActor:     true,
	packet.IDAddItemActor: true,
	packet.IDAddPainting:  true,
	packet.IDAddPlayer:    true,

	packet.IDBossEvent: true,

	packet.IDMobEffect: true,

	packet.IDPlayerList: true,

	packet.IDRemoveActor:     true,
	packet.IDRemoveObjective: true,

	packet.IDSetDisplayObjective: true,
}

type conn struct {
	addr       *net.UDPAddr
	conn       net.Conn
	compressor packet.Compression

	reader  *proto.Reader
	writer  *proto.Writer
	writeMu sync.Mutex

	clientData   login.ClientData
	identityData login.IdentityData

	gameData minecraft.GameData
	shieldID int32
	latency  int64

	pool   packet.Pool
	header packet.Header

	additionalPackets map[uint32]bool

	closed chan struct{}
}

func newConn(innerConn net.Conn, auth Authentication, additionalPackets map[uint32]bool, pool packet.Pool) (c *conn, err error) {
	c = &conn{
		conn:       innerConn,
		compressor: packet.FlateCompression,

		reader: proto.NewReader(innerConn),
		writer: proto.NewWriter(innerConn),

		pool:   pool,
		header: packet.Header{},

		additionalPackets: additionalPackets,

		closed: make(chan struct{}),
	}

	connectionRequestPacket, err := c.expect(packet2.IDConnectionRequest)
	if err != nil {
		_ = c.Close()
		return nil, err
	}

	connectionRequest, _ := connectionRequestPacket.(*packet2.ConnectionRequest)
	addr, err := net.ResolveUDPAddr("udp", connectionRequest.Addr)
	if err != nil {
		return nil, err
	}

	c.addr = addr
	if err := json.Unmarshal(connectionRequest.ClientData, &c.clientData); err != nil {
		_ = c.Close()
		return nil, err
	}

	if err := json.Unmarshal(connectionRequest.IdentityData, &c.identityData); err != nil {
		_ = c.Close()
		return nil, err
	}

	if auth != nil && !auth.Authenticate(c.identityData, connectionRequest.Token) {
		_ = c.Close()
		return nil, errors.New("authentication failed")
	}

	_ = c.WritePacket(&packet2.ConnectionResponse{
		RuntimeID: 1,
		UniqueID:  1,
	})
	return
}

// ReadPacket ...
func (c *conn) ReadPacket() (packet.Packet, error) {
	pk, err := c.read()
	if err != nil {
		return nil, err
	}

	if pk, ok := pk.(*packet2.Latency); ok {
		now := time.Now().UnixMilli()
		c.latency = (now - pk.Timestamp) + pk.Latency
		_ = c.WritePacket(&packet2.Latency{
			Timestamp: now,
			Latency:   c.latency,
		})
		return c.ReadPacket()
	}
	return pk, nil
}

// WritePacket ...
func (c *conn) WritePacket(pk packet.Packet) error {
	c.writeMu.Lock()
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		internal.BufferPool.Put(buf)
		c.writeMu.Unlock()
	}()

	c.header.PacketID = pk.ID()
	if err := c.header.Write(buf); err != nil {
		return err
	}

	pk.Marshal(protocol.NewWriter(buf, c.shieldID))
	data, err := c.compressor.Compress(buf.Bytes())
	if err != nil {
		return err
	}

	if decode, ok := packetsDecode[pk.ID()]; ok && decode {
		data = append([]byte{packetDecodeNeeded}, data...)
	} else if decode, ok := c.additionalPackets[pk.ID()]; ok && decode {
		data = append([]byte{packetDecodeNeeded}, data...)
	} else {
		data = append([]byte{packetDecodeNotNeeded}, data...)
	}

	return c.writer.Write(data)
}

// DecodePacket sets a packet decode-able based off the input packet and action given
func (c *conn) DecodePacket(id uint32, decode bool) {
	c.additionalPackets[id] = decode
}

// Flush ...
func (c *conn) Flush() error {
	return nil
}

// ClientData ...
func (c *conn) ClientData() login.ClientData {
	return c.clientData
}

// IdentityData ...
func (c *conn) IdentityData() login.IdentityData {
	return c.identityData
}

// ChunkRadius ...
func (c *conn) ChunkRadius() int {
	return 16
}

// ClientCacheEnabled ...
func (c *conn) ClientCacheEnabled() bool {
	return true
}

// RemoteAddr ...
func (c *conn) RemoteAddr() net.Addr {
	return c.addr
}

// Latency ...
func (c *conn) Latency() time.Duration {
	return time.Duration(c.latency)
}

// StartGameContext ...
func (c *conn) StartGameContext(_ context.Context, data minecraft.GameData) (err error) {
	for _, item := range data.Items {
		if item.Name == "minecraft:shield" {
			c.shieldID = int32(item.RuntimeID)
			break
		}
	}

	_ = c.WritePacket(&packet.StartGame{
		Difficulty:                   data.Difficulty,
		EntityUniqueID:               data.EntityUniqueID,
		EntityRuntimeID:              data.EntityRuntimeID,
		PlayerGameMode:               data.PlayerGameMode,
		PlayerPosition:               data.PlayerPosition,
		Pitch:                        data.Pitch,
		Yaw:                          data.Yaw,
		WorldSeed:                    data.WorldSeed,
		Dimension:                    data.Dimension,
		WorldSpawn:                   data.WorldSpawn,
		EditorWorldType:              data.EditorWorldType,
		CreatedInEditor:              data.CreatedInEditor,
		ExportedFromEditor:           data.ExportedFromEditor,
		PersonaDisabled:              data.PersonaDisabled,
		CustomSkinsDisabled:          data.CustomSkinsDisabled,
		GameRules:                    data.GameRules,
		Time:                         data.Time,
		Blocks:                       data.CustomBlocks,
		Items:                        data.Items,
		AchievementsDisabled:         true,
		Generator:                    1,
		EducationFeaturesEnabled:     true,
		MultiPlayerGame:              true,
		MultiPlayerCorrelationID:     uuid.Must(uuid.NewRandom()).String(),
		CommandsEnabled:              true,
		WorldName:                    data.WorldName,
		LANBroadcastEnabled:          true,
		PlayerMovementSettings:       data.PlayerMovementSettings,
		WorldGameMode:                data.WorldGameMode,
		ServerAuthoritativeInventory: data.ServerAuthoritativeInventory,
		PlayerPermissions:            data.PlayerPermissions,
		Experiments:                  data.Experiments,
		ClientSideGeneration:         data.ClientSideGeneration,
		ChatRestrictionLevel:         data.ChatRestrictionLevel,
		DisablePlayerInteractions:    data.DisablePlayerInteractions,
		BaseGameVersion:              data.BaseGameVersion,
		GameVersion:                  protocol.CurrentVersion,
		UseBlockNetworkIDHashes:      data.UseBlockNetworkIDHashes,
	})

	if _, err = c.expect(packet.IDRequestChunkRadius); err != nil {
		return err
	}

	_ = c.WritePacket(&packet.ChunkRadiusUpdated{ChunkRadius: 16})
	_ = c.WritePacket(&packet.PlayStatus{Status: packet.PlayStatusLoginSuccess})
	if _, err = c.expect(packet.IDSetLocalPlayerAsInitialised); err != nil {
		return err
	}
	return
}

// Close ...
func (c *conn) Close() (err error) {
	select {
	case <-c.closed:
		return errors.New("connection already closed")
	default:
		close(c.closed)
		_ = c.conn.Close()
		return
	}
}

// read reads a packet from the reader and returns it.
func (c *conn) read() (packet.Packet, error) {
	select {
	case <-c.closed:
		return nil, errors.New("connection closed")
	default:
		payload, err := c.reader.ReadPacket()
		if err != nil {
			return nil, err
		}

		decompressed, err := c.compressor.Decompress(payload)
		if err != nil {
			return nil, err
		}

		buf := internal.BufferPool.Get().(*bytes.Buffer)
		defer func() {
			buf.Reset()
			internal.BufferPool.Put(buf)

			if r := recover(); r != nil {
				err = fmt.Errorf("panic while reading packet: %v", r)
			}
		}()

		buf.Write(decompressed)
		header := packet.Header{}
		if err := header.Read(buf); err != nil {
			return nil, err
		}

		factory, ok := c.pool[header.PacketID]
		if !ok {
			return nil, fmt.Errorf("unknown packet ID %v", header.PacketID)
		}

		pk := factory()
		pk.Marshal(protocol.NewReader(buf, c.shieldID, false))
		return pk, nil
	}
}

// expect reads a packet from the connection and expects it to have the ID passed.
func (c *conn) expect(id uint32) (pk packet.Packet, err error) {
	pk, err = c.ReadPacket()
	if err != nil {
		return nil, err
	}

	if pk.ID() != id {
		return c.expect(id)
	}
	return
}
