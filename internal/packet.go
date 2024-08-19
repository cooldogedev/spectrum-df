package internal

import (
	"github.com/brentp/intintmap"
	packet2 "github.com/cooldogedev/spectrum/server/packet"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var internalPackets = []uint32{
	packet2.IDConnectionResponse,
	packet2.IDLatency,
	packet2.IDTransfer,

	packet.IDAddActor,
	packet.IDAddItemActor,
	packet.IDAddPainting,
	packet.IDAddPlayer,

	packet.IDBossEvent,

	packet.IDChunkRadiusUpdated,

	packet.IDMobEffect,

	packet.IDPlayerList,
	packet.IDPlayStatus,

	packet.IDRemoveActor,
	packet.IDRemoveObjective,

	packet.IDSetDisplayObjective,
	packet.IDStartGame,
}

var PacketMap *intintmap.Map

func PacketExists(packet uint32) bool {
	_, ok := PacketMap.Get(int64(packet))
	return ok
}

func init() {
	PacketMap = intintmap.New(len(internalPackets), 0.999)
	for _, id := range internalPackets {
		PacketMap.Put(int64(id), 1)
	}
}
