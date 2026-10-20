package runtime

import (
	"context"
	"errors"
)

var (
	ErrProviderNil           = errors.New("runtime: provider is nil")
	ErrProviderNameRequired  = errors.New("runtime: provider name is required")
	ErrProviderNameReserved  = errors.New("runtime: provider name is reserved")
	ErrProviderNameDuplicate = errors.New("runtime: provider name is duplicate")
	ErrProviderNotFound      = errors.New("runtime: provider not found")
	ErrProviderServiceExited = errors.New("runtime: service provider exited before runtime stopped")
)

type ProviderMode string

const (
	ProviderModeService ProviderMode = "service"
	ProviderModeJob     ProviderMode = "job"
)

type ProviderContract interface {
	Name() string
	Metadata() ProviderMetadata
	Dependencies() []Dependency
	Register(app *App) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type Provider interface {
	ProviderContract
	Status() Status
}

type RuntimeCapabilityProvider interface {
	RuntimeCapabilities() []CapabilityContract
}

func providerRuntimeCapabilities(provider ProviderContract) []CapabilityContract {
	if provider == nil {
		return nil
	}
	if wrapped, ok := provider.(*runtimeProvider); ok {
		provider = wrapped.ProviderContract
		if provider == nil {
			return nil
		}
	}
	capabilityProvider, ok := provider.(RuntimeCapabilityProvider)
	if !ok {
		return nil
	}
	capabilities := capabilityProvider.RuntimeCapabilities()
	if len(capabilities) == 0 {
		return nil
	}
	out := make([]CapabilityContract, len(capabilities))
	copy(out, capabilities)
	return out
}

func isReservedProviderName(name string) bool {
	switch name {
	case "provider", "default", "main":
		return true
	default:
		return false
	}
}

// ReadinessProvider reports when a blocking service can accept work.
// A nil or open channel is not ready. A service must close it only after initialization.
type ReadinessProvider interface{ Ready() <-chan struct{} }

func providerReady(provider ProviderContract) bool {
	ready, ok := provider.(ReadinessProvider)
	if !ok {
		return false
	}
	select {
	case <-ready.Ready():
		return true
	default:
		return false
	}
}
