package ratelimit

import (
	"sync"

	"github.com/huwenlong92/sdkit-go/pkg/ratelimit/store"
)

// CustomStore is the shared store used by framework adapters.
var CustomStore store.Store // Deprecated: use StoreDefault and SetStore.
var defaultMu sync.RWMutex

// SetStore sets the shared rate limit store. Passing nil restores adapter defaults.
func SetStore(s store.Store) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	CustomStore = s
}

func PickStore() store.Store {
	if current := StoreDefault(); current != nil {
		return current
	}
	return store.NewMemoryStore()
}

func StoreDefault() store.Store { defaultMu.RLock(); defer defaultMu.RUnlock(); return CustomStore }
