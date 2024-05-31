package util

import (
	"github.com/cooldogedev/spectrum/server/packet"
	packet2 "github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var decodeMap = map[uint32]bool{}

func RegisterPacketDecode(id uint32, value bool) {
	decodeMap[id] = value
}

func init() {
	RegisterPacketDecode(packet.IDLatency, true)
	RegisterPacketDecode(packet.IDTransfer, true)

	RegisterPacketDecode(packet2.IDAddActor, true)
	RegisterPacketDecode(packet2.IDAddItemActor, true)
	RegisterPacketDecode(packet2.IDAddPainting, true)
	RegisterPacketDecode(packet2.IDAddPlayer, true)

	RegisterPacketDecode(packet2.IDBossEvent, true)

	RegisterPacketDecode(packet2.IDMobEffect, true)

	RegisterPacketDecode(packet2.IDPlayerList, true)

	RegisterPacketDecode(packet2.IDRemoveActor, true)
	RegisterPacketDecode(packet2.IDRemoveObjective, true)

	RegisterPacketDecode(packet2.IDSetDisplayObjective, true)
}
