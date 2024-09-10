package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"sync"
	"time"

	proto "github.com/cooldogedev/spectrum/protocol"
	packet2 "github.com/cooldogedev/spectrum/server/packet"
	"github.com/golang/snappy"
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

type Conn struct {
	addr *net.UDPAddr
	conn io.ReadWriteCloser

	reader *proto.Reader
	writer *proto.Writer

	clientData   login.ClientData
	identityData login.IdentityData

	gameData  minecraft.GameData
	runtimeID uint64
	uniqueID  int64

	shieldID int32
	latency  int64

	header *packet.Header
	pool   packet.Pool

	ch chan struct{}
	mu sync.Mutex
}

func NewConn(conn io.ReadWriteCloser, authenticator Authenticator, pool packet.Pool) (*Conn, error) {
	c := &Conn{
		conn: conn,

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

	if authenticator != nil && !authenticator(c.identityData, connectionRequest.Token) {
		_ = c.Close()
		return nil, errors.New("authentication failed")
	}

	c.runtimeID = uint64(crc32.ChecksumIEEE([]byte(c.identityData.XUID)))
	c.uniqueID = int64(c.runtimeID)
	if err := c.WritePacket(&packet2.ConnectionResponse{RuntimeID: c.runtimeID, UniqueID: c.uniqueID}); err != nil {
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

	pk = c.translatePacket(pk, true)
	c.header.PacketID = pk.ID()
	if err := c.header.Write(buf); err != nil {
		return err
	}

	var decodeByte byte
	if PacketExists(pk.ID()) {
		decodeByte = packetDecodeNeeded
	} else {
		decodeByte = packetDecodeNotNeeded
	}
	pk.Marshal(protocol.NewWriter(buf, c.shieldID))
	return c.writer.Write(append([]byte{decodeByte}, snappy.Encode(nil, buf.Bytes())...))
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
	return false
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
		EntityUniqueID:               c.uniqueID,
		EntityRuntimeID:              c.runtimeID,
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

		decompressed, err := snappy.Decode(nil, payload)
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
		return c.translatePacket(pk, false), nil
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

// translatePacket processes and translates entity identifiers in the given packet.
// It converts runtime and unique IDs between client and server representations depending
// on the direction of the packet.
func (c *Conn) translatePacket(pk packet.Packet, serverSent bool) packet.Packet {
	switch pk := pk.(type) {
	case *packet.ActorEvent:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.ActorPickRequest:
		pk.EntityUniqueID = c.translateUniqueID(pk.EntityUniqueID, serverSent)
	case *packet.AddActor:
		pk.EntityUniqueID = c.translateUniqueID(pk.EntityUniqueID, serverSent)
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
		pk.EntityMetadata = c.translateMetadata(pk.EntityMetadata, serverSent)
		for i := range pk.EntityLinks {
			pk.EntityLinks[i] = c.translateLink(pk.EntityLinks[i], serverSent)
		}
	case *packet.AddItemActor:
		pk.EntityUniqueID = c.translateUniqueID(pk.EntityUniqueID, serverSent)
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
		pk.EntityMetadata = c.translateMetadata(pk.EntityMetadata, serverSent)
	case *packet.AddPainting:
		pk.EntityUniqueID = c.translateUniqueID(pk.EntityUniqueID, serverSent)
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.AddPlayer:
		pk.AbilityData.EntityUniqueID = c.translateUniqueID(pk.AbilityData.EntityUniqueID, serverSent)
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
		pk.EntityMetadata = c.translateMetadata(pk.EntityMetadata, serverSent)
		for i := range pk.EntityLinks {
			pk.EntityLinks[i] = c.translateLink(pk.EntityLinks[i], serverSent)
		}
		pk.AbilityData.EntityUniqueID = c.translateUniqueID(pk.AbilityData.EntityUniqueID, serverSent)
	case *packet.AddVolumeEntity:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.AdventureSettings:
		pk.PlayerUniqueID = c.translateUniqueID(pk.PlayerUniqueID, serverSent)
	case *packet.AgentAnimation:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.Animate:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.AnimateEntity:
		for i := range pk.EntityRuntimeIDs {
			pk.EntityRuntimeIDs[i] = c.translateRuntimeID(pk.EntityRuntimeIDs[i], serverSent)
		}
	case *packet.BossEvent:
		pk.BossEntityUniqueID = c.translateUniqueID(pk.BossEntityUniqueID, serverSent)
		pk.PlayerUniqueID = c.translateUniqueID(pk.PlayerUniqueID, serverSent)
	case *packet.Camera:
		pk.CameraEntityUniqueID = c.translateUniqueID(pk.CameraEntityUniqueID, serverSent)
		pk.TargetPlayerUniqueID = c.translateUniqueID(pk.TargetPlayerUniqueID, serverSent)
	case *packet.ChangeMobProperty:
		pk.EntityUniqueID = c.translateRuntimeID(pk.EntityUniqueID, serverSent)
	case *packet.ClientBoundMapItemData:
		for i, x := range pk.TrackedObjects {
			if x.Type == protocol.MapObjectTypeEntity {
				x.EntityUniqueID = c.translateUniqueID(x.EntityUniqueID, serverSent)
				pk.TrackedObjects[i] = x
			}
		}
	case *packet.ClientCheatAbility:
		pk.AbilityData.EntityUniqueID = c.translateUniqueID(pk.AbilityData.EntityUniqueID, serverSent)
	case *packet.CommandBlockUpdate:
		if !pk.Block {
			pk.MinecartEntityRuntimeID = c.translateRuntimeID(pk.MinecartEntityRuntimeID, serverSent)
		}
	case *packet.CommandOutput:
		pk.CommandOrigin.PlayerUniqueID = c.translateUniqueID(pk.CommandOrigin.PlayerUniqueID, serverSent)
	case *packet.CommandRequest:
		pk.CommandOrigin.PlayerUniqueID = c.translateUniqueID(pk.CommandOrigin.PlayerUniqueID, serverSent)
	case *packet.ContainerOpen:
		pk.ContainerEntityUniqueID = c.translateUniqueID(pk.ContainerEntityUniqueID, serverSent)
	case *packet.CreatePhoto:
		pk.EntityUniqueID = c.translateUniqueID(pk.EntityUniqueID, serverSent)
	case *packet.DebugInfo:
		pk.PlayerUniqueID = c.translateUniqueID(pk.PlayerUniqueID, serverSent)
	case *packet.Emote:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.EmoteList:
		pk.PlayerRuntimeID = c.translateRuntimeID(pk.PlayerRuntimeID, serverSent)
	case *packet.Event:
		pk.EntityRuntimeID = int64(c.translateRuntimeID(uint64(pk.EntityRuntimeID), serverSent))
		switch data := pk.Event.(type) {
		case *protocol.MobKilledEvent:
			data.KillerEntityUniqueID = c.translateUniqueID(data.KillerEntityUniqueID, serverSent)
			data.VictimEntityUniqueID = c.translateUniqueID(data.VictimEntityUniqueID, serverSent)
		case *protocol.BossKilledEvent:
			data.BossEntityUniqueID = c.translateUniqueID(data.BossEntityUniqueID, serverSent)
		}
	case *packet.Interact:
		pk.TargetEntityRuntimeID = c.translateRuntimeID(pk.TargetEntityRuntimeID, serverSent)
	case *packet.InventoryTransaction:
		switch data := pk.TransactionData.(type) {
		case *protocol.UseItemOnEntityTransactionData:
			data.TargetEntityRuntimeID = c.translateRuntimeID(data.TargetEntityRuntimeID, serverSent)
		}
	case *packet.MobArmourEquipment:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.MobEffect:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.MobEquipment:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.MotionPredictionHints:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.MoveActorAbsolute:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.MoveActorDelta:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.MovePlayer:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
		pk.RiddenEntityRuntimeID = c.translateRuntimeID(pk.RiddenEntityRuntimeID, serverSent)
	case *packet.NPCDialogue:
		pk.EntityUniqueID = uint64(c.translateUniqueID(int64(pk.EntityUniqueID), serverSent))
	case *packet.NPCRequest:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.PhotoTransfer:
		pk.OwnerEntityUniqueID = c.translateUniqueID(pk.OwnerEntityUniqueID, serverSent)
	case *packet.PlayerAction:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.PlayerAuthInput:
		if pk.InputData&packet.InputFlagClientPredictedVehicle != 0 {
			pk.ClientPredictedVehicle = c.translateUniqueID(pk.ClientPredictedVehicle, serverSent)
		}
	case *packet.PlayerList:
		for i := range pk.Entries {
			pk.Entries[i].EntityUniqueID = c.translateUniqueID(pk.Entries[i].EntityUniqueID, serverSent)
		}
	case *packet.RemoveActor:
		pk.EntityUniqueID = c.translateUniqueID(pk.EntityUniqueID, serverSent)
	case *packet.RemoveVolumeEntity:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.Respawn:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.SetActorData:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
		pk.EntityMetadata = c.translateMetadata(pk.EntityMetadata, serverSent)
	case *packet.SetActorLink:
		pk.EntityLink = c.translateLink(pk.EntityLink, serverSent)
	case *packet.SetActorMotion:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.SetLocalPlayerAsInitialised:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.SetScore:
		for i := range pk.Entries {
			if pk.Entries[i].IdentityType != protocol.ScoreboardIdentityFakePlayer {
				pk.Entries[i].EntityUniqueID = c.translateUniqueID(pk.Entries[i].EntityUniqueID, serverSent)
			}
		}
	case *packet.SetScoreboardIdentity:
		if pk.ActionType != packet.ScoreboardIdentityActionClear {
			for i := range pk.Entries {
				pk.Entries[i].EntityUniqueID = c.translateUniqueID(pk.Entries[i].EntityUniqueID, serverSent)
			}
		}
	case *packet.ShowCredits:
		pk.PlayerRuntimeID = c.translateRuntimeID(pk.PlayerRuntimeID, serverSent)
	case *packet.SpawnParticleEffect:
		pk.EntityUniqueID = c.translateUniqueID(pk.EntityUniqueID, serverSent)
	case *packet.StartGame:
		pk.EntityUniqueID = c.translateUniqueID(pk.EntityUniqueID, serverSent)
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.StructureBlockUpdate:
		pk.Settings.LastEditingPlayerUniqueID = c.translateUniqueID(pk.Settings.LastEditingPlayerUniqueID, serverSent)
	case *packet.StructureTemplateDataRequest:
		pk.Settings.LastEditingPlayerUniqueID = c.translateUniqueID(pk.Settings.LastEditingPlayerUniqueID, serverSent)
	case *packet.TakeItemActor:
		pk.ItemEntityRuntimeID = c.translateRuntimeID(pk.ItemEntityRuntimeID, serverSent)
		pk.TakerEntityRuntimeID = c.translateRuntimeID(pk.TakerEntityRuntimeID, serverSent)
	case *packet.UpdateAbilities:
		pk.AbilityData.EntityUniqueID = c.translateUniqueID(pk.AbilityData.EntityUniqueID, serverSent)
	case *packet.UpdateAttributes:
		pk.EntityRuntimeID = c.translateRuntimeID(pk.EntityRuntimeID, serverSent)
	case *packet.UpdateBlockSynced:
		pk.EntityUniqueID = uint64(c.translateUniqueID(int64(pk.EntityUniqueID), serverSent))
	case *packet.UpdateEquip:
		pk.EntityUniqueID = c.translateUniqueID(pk.EntityUniqueID, serverSent)
	case *packet.UpdatePlayerGameType:
		pk.PlayerUniqueID = c.translateUniqueID(pk.PlayerUniqueID, serverSent)
	case *packet.UpdateSubChunkBlocks:
		for i, entry := range pk.Blocks {
			pk.Blocks[i].SyncedUpdateEntityUniqueID = uint64(c.translateUniqueID(int64(entry.SyncedUpdateEntityUniqueID), serverSent))
		}
		for i, entry := range pk.Extra {
			pk.Extra[i].SyncedUpdateEntityUniqueID = uint64(c.translateUniqueID(int64(entry.SyncedUpdateEntityUniqueID), serverSent))
		}
	case *packet.UpdateTrade:
		pk.VillagerUniqueID = c.translateUniqueID(pk.VillagerUniqueID, serverSent)
		pk.EntityUniqueID = c.translateUniqueID(pk.EntityUniqueID, serverSent)
	}
	return pk
}

// translateRuntimeID converts a runtime ID based on whether the packet was sent by the server or by the client.
// It converts the client-side runtime ID to the server-side runtime ID and vice versa based on the packet direction.
func (c *Conn) translateRuntimeID(runtimeId uint64, serverSent bool) uint64 {
	search := c.runtimeID
	replace := uint64(1)
	if serverSent {
		search = uint64(1)
		replace = c.runtimeID
	}

	if runtimeId == search {
		return replace
	}
	return runtimeId
}

// translateUniqueID converts a unique ID based on whether the packet was sent by the server or by the client.
// It converts the client-side unique ID to the server-side unique ID and vice versa based on the packet direction.
func (c *Conn) translateUniqueID(runtimeId int64, serverSent bool) int64 {
	search := c.uniqueID
	replace := int64(1)
	if serverSent {
		search = int64(1)
		replace = c.uniqueID
	}

	if runtimeId == search {
		return replace
	}
	return runtimeId
}

// translateMetadata updates entity metadata fields that contain unique IDs or runtime IDs,
// translating them based the packet direction.
func (c *Conn) translateMetadata(metadata map[uint32]any, serverSent bool) map[uint32]any {
	for key, value := range metadata {
		switch key {
		case protocol.EntityDataKeyOwner:
			metadata[protocol.EntityDataKeyOwner] = c.translateUniqueID(value.(int64), serverSent)
		case protocol.EntityDataKeyTarget:
			metadata[key] = c.translateUniqueID(value.(int64), serverSent)
		case protocol.EntityDataKeyDisplayOffset:
			metadata[key] = c.translateUniqueID(value.(int64), serverSent)
		case protocol.EntityDataKeyLeashHolder:
			metadata[key] = c.translateUniqueID(value.(int64), serverSent)
		case protocol.EntityDataKeyAgent:
			metadata[key] = c.translateUniqueID(value.(int64), serverSent)
		case protocol.EntityDataKeyBaseRuntimeID:
			metadata[key] = c.translateRuntimeID(value.(uint64), serverSent)
		default:
		}
	}
	return metadata
}

// translateLink updates an entity link by translating the unique IDs of the rider and the ridden entities,
// based on the packet direction.
func (c *Conn) translateLink(link protocol.EntityLink, serverSent bool) protocol.EntityLink {
	link.RiderEntityUniqueID = c.translateUniqueID(link.RiderEntityUniqueID, serverSent)
	link.RiddenEntityUniqueID = c.translateUniqueID(link.RiddenEntityUniqueID, serverSent)
	return link
}
