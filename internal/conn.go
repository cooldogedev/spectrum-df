package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
	_ "unsafe"

	"github.com/cooldogedev/spectrum-df/util"
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

//go:linkname decodeMap spectrum.decodeMap
var decodeMap map[uint32]bool

type Conn struct {
	addr       *net.UDPAddr
	conn       io.ReadWriteCloser
	compressor packet.Compression

	reader  *proto.Reader
	writer  *proto.Writer
	writeMu sync.Mutex

	clientData   login.ClientData
	identityData login.IdentityData

	gameData minecraft.GameData
	shieldID int32
	latency  int64

	header packet.Header
	pool   packet.Pool

	closed chan struct{}
}

func NewConn(conn io.ReadWriteCloser, authentication util.Authentication, pool packet.Pool) (c *Conn, err error) {
	c = &Conn{
		conn:       conn,
		compressor: packet.FlateCompression,

		reader: proto.NewReader(conn),
		writer: proto.NewWriter(conn),

		header: packet.Header{},
		pool:   pool,

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
		_ = c.Close()
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

	if authentication != nil && !authentication.Authenticate(c.identityData, connectionRequest.Token) {
		_ = c.Close()
		return nil, errors.New("authentication failed")
	}

	if err := c.WritePacket(&packet2.ConnectionResponse{RuntimeID: 1, UniqueID: 1}); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

// ReadPacket ...
func (c *Conn) ReadPacket() (packet.Packet, error) {
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
func (c *Conn) WritePacket(pk packet.Packet) error {
	c.writeMu.Lock()
	buf := BufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		BufferPool.Put(buf)
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

	if decode, ok := decodeMap[pk.ID()]; ok && decode {
		data = append([]byte{packetDecodeNeeded}, data...)
	} else {
		data = append([]byte{packetDecodeNotNeeded}, data...)
	}
	return c.writer.Write(data)
}

// Flush ...
func (c *Conn) Flush() error {
	return nil
}

// ClientData ...
func (c *Conn) ClientData() login.ClientData {
	return c.clientData
}

// IdentityData ...
func (c *Conn) IdentityData() login.IdentityData {
	return c.identityData
}

// ChunkRadius ...
func (c *Conn) ChunkRadius() int {
	return 16
}

// ClientCacheEnabled ...
func (c *Conn) ClientCacheEnabled() bool {
	return true
}

// RemoteAddr ...
func (c *Conn) RemoteAddr() net.Addr {
	return c.addr
}

// Latency ...
func (c *Conn) Latency() time.Duration {
	return time.Duration(c.latency)
}

// StartGameContext ...
func (c *Conn) StartGameContext(_ context.Context, data minecraft.GameData) (err error) {
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
func (c *Conn) Close() (err error) {
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
func (c *Conn) read() (packet.Packet, error) {
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

		buf := BufferPool.Get().(*bytes.Buffer)
		defer func() {
			buf.Reset()
			BufferPool.Put(buf)

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
func (c *Conn) expect(id uint32) (pk packet.Packet, err error) {
	pk, err = c.ReadPacket()
	if err != nil {
		return nil, err
	}

	if pk.ID() != id {
		return c.expect(id)
	}
	return
}
