package runtime

import (
	"errors"
	"reflect"
	"sync"
)

type LocalCapability interface {
	Name() string
}

type localCapabilityCloser interface {
	Close() error
}

type LocalCapabilityRegistry struct {
	mu        sync.RWMutex
	instances map[string]any
	owned     []localCapabilityCloser
	names     []string
}

func NewLocalCapabilityRegistry() *LocalCapabilityRegistry {
	return &LocalCapabilityRegistry{}
}

func (r *LocalCapabilityRegistry) Add(capability LocalCapability) {
	if r == nil || capability == nil {
		return
	}
	r.AddName(capability.Name())
}

func (r *LocalCapabilityRegistry) AddName(name string) {
	if r == nil || name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.addNameLocked(name)
}

func (r *LocalCapabilityRegistry) Set(name string, instance any) {
	if r == nil || name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.addNameLocked(name)
	if instance == nil {
		return
	}
	if r.instances == nil {
		r.instances = make(map[string]any)
	}
	r.instances[name] = instance
	// Keep ownership until Close even if a later registration replaces this name.
	// Previously returned instances may still be in use by the service.
	if closer, ok := instance.(localCapabilityCloser); ok && !isNilCapabilityValue(closer) {
		r.owned = append(r.owned, closer)
	}
}

// SetBorrowed exposes an instance whose lifecycle belongs to its runtime capability.
// It does not relinquish ownership of resources previously registered with Set.
func (r *LocalCapabilityRegistry) SetBorrowed(name string, instance any) {
	if r == nil || name == "" || isNilCapabilityValue(instance) {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.addNameLocked(name)
	if r.instances == nil {
		r.instances = make(map[string]any)
	}
	r.instances[name] = instance
}

func (r *LocalCapabilityRegistry) Get(name string) (any, bool) {
	if r == nil || name == "" {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, ok := r.instances[name]
	return value, ok
}

func LocalCapabilityAs[T any](registry *LocalCapabilityRegistry, name string) (T, bool) {
	var zero T
	if registry == nil {
		return zero, false
	}
	value, ok := registry.Get(name)
	if !ok {
		return zero, false
	}
	typed, ok := value.(T)
	if !ok {
		return zero, false
	}
	return typed, true
}

func (r *LocalCapabilityRegistry) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	owned := r.owned
	r.instances = nil
	r.owned = nil
	r.mu.Unlock()

	var errs []error
	closed := make(map[any]struct{})
	for i := len(owned) - 1; i >= 0; i-- {
		closer := owned[i]
		if reflect.TypeOf(closer).Comparable() {
			if _, seen := closed[closer]; seen {
				continue
			}
			closed[closer] = struct{}{}
		}
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (r *LocalCapabilityRegistry) addNameLocked(name string) {
	r.names = append(r.names, name)
}

func (r *LocalCapabilityRegistry) Names() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.names) == 0 {
		return nil
	}
	names := make([]string, 0, len(r.names))
	seen := make(map[string]struct{}, len(r.names))
	for _, name := range r.names {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}
