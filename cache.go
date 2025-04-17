package spectrum

import "sync"

type cacheEntry struct {
	data       []byte
	protocolID int32
}

var (
	cache   map[string]cacheEntry
	cacheMu sync.RWMutex
)

func GetCache(xuid string) ([]byte, int32) {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	if c, ok := cache[xuid]; ok {
		return c.data, c.protocolID
	}
	return nil, 0
}

func setCache(xuid string, data []byte, protocolID int32) {
	cacheMu.Lock()
	cache[xuid] = cacheEntry{data: data, protocolID: protocolID}
	cacheMu.Unlock()
}

func deleteCache(xuid string) {
	cacheMu.Lock()
	delete(cache, xuid)
	cacheMu.Unlock()
}

func init() {
	cache = make(map[string]cacheEntry)
}
