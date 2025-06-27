package spectrum

import (
	"github.com/brentp/intintmap"
	spectrumpacket "github.com/cooldogedev/spectrum/server/packet"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var internalPackets = []uint32{
	spectrumpacket.IDConnectionResponse,
	spectrumpacket.IDFlush,
	spectrumpacket.IDLatency,
	spectrumpacket.IDTransfer,
	spectrumpacket.IDUpdateCache,

	packet.IDAddActor,
	packet.IDAddItemActor,
	packet.IDAddPainting,
	packet.IDAddPlayer,

	packet.IDBossEvent,

	packet.IDChunkRadiusUpdated,

	packet.IDItemRegistry,

	packet.IDMobEffect,

	packet.IDPlayerList,
	packet.IDPlayStatus,

	packet.IDRemoveActor,
	packet.IDRemoveObjective,

	packet.IDSetDisplayObjective,
	packet.IDStartGame,
}

var packetMap *intintmap.Map

func RegisterPacketDecode(id uint32, value bool) {
	if value {
		packetMap.Put(int64(id), 1)
	} else {
		packetMap.Del(int64(id))
	}
}

func shouldDecodePacket(packet uint32) bool {
	_, ok := packetMap.Get(int64(packet))
	return ok
}

func init() {
	packetMap = intintmap.New(len(internalPackets), 0.999)
	for _, id := range internalPackets {
		packetMap.Put(int64(id), 1)
	}
}
