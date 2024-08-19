package util

import "github.com/cooldogedev/spectrum-df/internal"

func RegisterPacketDecode(id uint32, value bool) {
	if value {
		internal.PacketMap.Put(int64(id), 1)
	} else {
		internal.PacketMap.Del(int64(id))
	}
}
