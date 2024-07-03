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
	packetDecodeNeeded byte = iota
	packetDecodeNotNeeded
)

//go:linkname decodeMap spectrum.decodeMap
var decodeMap map[uint32]bool

type Conn struct {
	addr       *net.UDPAddr
	conn       io.ReadWriteCloser
	compressor packet.Compression

	reader *proto.Reader
	writer *proto.Writer

	clientData   login.ClientData
	identityData login.IdentityData

	gameData minecraft.GameData
	shieldID int32
	latency  int64

	header *packet.Header
	pool   packet.Pool

	ch chan struct{}
	mu sync.Mutex
}

func NewConn(conn io.ReadWriteCloser, authentication util.Authentication, pool packet.Pool) (*Conn, error) {
	c := &Conn{
		conn:       conn,
		compressor: packet.FlateCompression,

		reader: proto.NewReader(conn),
		writer: proto.NewWriter(conn),

		header: &packet.Header{},
		pool:   pool,

		ch: make(chan struct{}),
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
		c.latency = (time.Now().UnixMilli() - pk.Timestamp) + pk.Latency
		_ = c.WritePacket(&packet2.Latency{Timestamp: 0, Latency: c.latency})
		return c.ReadPacket()
	}
	return pk, nil
}

// WritePacket ...
func (c *Conn) WritePacket(pk packet.Packet) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	buf := BufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		BufferPool.Put(buf)
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

	decodeByte := packetDecodeNotNeeded
	if decode, ok := decodeMap[pk.ID()]; ok && decode {
		decodeByte = packetDecodeNeeded
	}
	return c.writer.Write(append([]byte{decodeByte}, data...))
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

	startGame := &packet.StartGame{
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
	}
	if err = c.WritePacket(startGame); err != nil {
		return err
	}

	if _, err = c.expect(packet.IDRequestChunkRadius); err != nil {
		return err
	}

	if err := c.WritePacket(&packet.ChunkRadiusUpdated{ChunkRadius: 16}); err != nil {
		return err
	}

	if err := c.WritePacket(&packet.PlayStatus{Status: packet.PlayStatusLoginSuccess}); err != nil {
		return err
	}

	if _, err = c.expect(packet.IDSetLocalPlayerAsInitialised); err != nil {
		return err
	}
	return
}

// Close ...
func (c *Conn) Close() (err error) {
	select {
	case <-c.ch:
		return errors.New("connection already closed")
	default:
		close(c.ch)
		_ = c.conn.Close()
		return
	}
}

// read reads a packet from the reader and returns it.
func (c *Conn) read() (packet.Packet, error) {
	select {
	case <-c.ch:
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

		buf := bytes.NewBuffer(decompressed)
		header := &packet.Header{}
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
func (c *Conn) expect(id uint32) (packet.Packet, error) {
	pk, err := c.ReadPacket()
	if err != nil {
		return nil, err
	}

	if pk.ID() == id {
		return pk, nil
	}
	return c.expect(id)
}
