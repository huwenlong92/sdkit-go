package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var ErrAppNil = errors.New("runtime: app is nil")
var ErrAppRunning = errors.New("runtime: app run already in progress")

type App struct {
	defaultReleases    []func()
	defaultReleaseDone chan struct{}
	defaultKeys        []Key
	container          *Container
	registry           *Registry
	ctx                context.Context
	cancel             context.CancelFunc

	runMu                   sync.Mutex
	lifecycleGate           chan struct{}
	mu                      sync.Mutex
	registeredCapabilities  []Capability
	providerCapabilityNames map[string]struct{}
	runningProviders        []Provider
	// stopPending records unfinished cleanup; stopping records an active Stop attempt.
	stopPending bool
	stopping    bool
	stopped     bool
	stopErr     error
}

func New() *App {
	ctx, cancel := context.WithCancel(context.Background())
	return &App{
		container: NewContainer(),
		registry:  NewRegistry(),
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (a *App) Context() context.Context {
	if a == nil {
		return context.Background()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ctx == nil {
		a.ctx, a.cancel = context.WithCancel(context.Background())
	}
	return a.ctx
}

func (a *App) Container() *Container {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.container == nil {
		a.container = NewContainer()
	}
	return a.container
}

func (a *App) registryRef() *Registry {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.registry == nil {
		a.registry = NewRegistry()
	}
	return a.registry
}

func (a *App) Use(capabilities ...CapabilityContract) error {
	return a.RegisterCapabilities(capabilities...)
}

func (a *App) RegisterCapabilities(capabilities ...CapabilityContract) error {
	if a == nil {
		return ErrAppNil
	}
	return a.registryRef().RegisterCapabilities(capabilities...)
}

func (a *App) Register(providers ...ProviderContract) error {
	if a == nil {
		return ErrAppNil
	}
	return a.registryRef().RegisterProviders(providers...)
}

func (a *App) RegisterCommand(commands ...Command) error {
	if a == nil {
		return ErrAppNil
	}
	return a.registryRef().RegisterCommands(commands...)
}

func (a *App) RegisterAdapters(adapters ...Adapter) error {
	if a == nil {
		return ErrAppNil
	}
	return a.registryRef().RegisterAdapters(adapters...)
}

func (a *App) UseCapabilityAdapters(adapters ...CapabilityAdapter) error {
	if a == nil {
		return ErrAppNil
	}
	genericAdapters := make([]Adapter, 0, len(adapters))
	capabilities := make([]CapabilityContract, 0, len(adapters))
	for _, adapter := range adapters {
		if adapter == nil {
			return ErrAdapterNil
		}
		capability := adapter.Capability()
		if capability == nil {
			return ErrCapabilityNil
		}
		genericAdapters = append(genericAdapters, adapter)
		capabilities = append(capabilities, capability)
	}
	if err := a.registryRef().RegisterAdapters(genericAdapters...); err != nil {
		return err
	}
	return a.Use(capabilities...)
}

func (a *App) RegisterProviderAdapters(adapters ...ProviderAdapter) error {
	if a == nil {
		return ErrAppNil
	}
	genericAdapters := make([]Adapter, 0, len(adapters))
	providers := make([]ProviderContract, 0, len(adapters))
	for _, adapter := range adapters {
		if adapter == nil {
			return ErrAdapterNil
		}
		provider := adapter.Provider()
		if provider == nil {
			return ErrProviderNil
		}
		genericAdapters = append(genericAdapters, adapter)
		providers = append(providers, provider)
	}
	if err := a.registryRef().RegisterAdapters(genericAdapters...); err != nil {
		return err
	}
	return a.Register(providers...)
}

func (a *App) RegisterCommandAdapters(adapters ...CommandAdapter) error {
	if a == nil {
		return ErrAppNil
	}
	genericAdapters := make([]Adapter, 0, len(adapters))
	commands := make([]Command, 0, len(adapters))
	for _, adapter := range adapters {
		if adapter == nil {
			return ErrAdapterNil
		}
		command := adapter.Command()
		if command == nil {
			return ErrCommandNil
		}
		genericAdapters = append(genericAdapters, adapter)
		commands = append(commands, command)
	}
	if err := a.registryRef().RegisterAdapters(genericAdapters...); err != nil {
		return err
	}
	return a.RegisterCommand(commands...)
}

func (a *App) RegisterPlugins(plugins ...Plugin) error {
	if a == nil {
		return ErrAppNil
	}
	return a.registryRef().RegisterPlugins(plugins...)
}

func (a *App) Run(ctxs ...context.Context) error {
	return a.RunAllProviders(ctxs...)
}

func (a *App) RunAllProviders(ctxs ...context.Context) (runErr error) {
	if a == nil {
		return ErrAppNil
	}
	if !a.runMu.TryLock() {
		return ErrAppRunning
	}
	defer a.runMu.Unlock()
	// A completed job run can be invoked again; release its previous resources first.
	a.mu.Lock()
	previousRun := a.stopped || a.defaultReleaseDone != nil || len(a.registeredCapabilities) > 0 || len(a.runningProviders) > 0
	a.mu.Unlock()
	if previousRun {
		if err := a.Stop(context.Background()); err != nil {
			return err
		}
	}
	defer func() {
		if runErr != nil {
			failedProviders := a.ProviderStatuses()
			runErr = errors.Join(runErr, a.Stop(context.Background()))
			for _, health := range failedProviders {
				if health.Status == StatusFailed {
					if provider, ok := a.Provider(health.Name); ok {
						setObjectStatus(provider, StatusFailed, health.Error)
					}
				}
			}
		}
	}()
	ctx := a.prepareRunContext(ctxs...)
	stopSignals := a.watchSignals(ctx)
	defer stopSignals()
	release, err := a.lockLifecycle(ctx)
	if err != nil {
		return err
	}
	// This defer must run before the earlier rollback defer calls Stop,
	// which acquires the same lifecycle gate on registration failure.
	defer release()

	providers := a.Providers()
	if err := a.registerProviderCapabilities(providers); err != nil {
		a.failRuntimeStatuses(err)
		return err
	}

	capabilities, err := SortCapabilities(a.Capabilities())
	if err != nil {
		a.failRuntimeStatuses(err)
		return err
	}
	providers, err = SortProviders(providers, capabilities)
	if err != nil {
		a.failRuntimeStatuses(err)
		return err
	}

	for _, capability := range capabilities {
		if err := ctx.Err(); err != nil {
			return err
		}
		setObjectStatus(capability, StatusBooting, nil)
		if err := capability.Register(a); err != nil {
			setObjectStatus(capability, StatusFailed, err)
			return err
		}
		setObjectStatus(capability, StatusRunning, nil)
		a.addRegisteredCapability(capability)
	}
	for _, provider := range providers {
		if err := ctx.Err(); err != nil {
			return err
		}
		setObjectStatus(provider, StatusBooting, nil)
		if err := provider.Register(a); err != nil {
			setObjectStatus(provider, StatusFailed, err)
			return err
		}
		a.addRunningProvider(provider)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	release()
	return a.startProviders(ctx, providers)
}

func (a *App) RunProvider(name string, ctxs ...context.Context) (runErr error) {
	if a == nil {
		return ErrAppNil
	}
	if _, ok := a.Provider(name); !ok {
		return fmt.Errorf("%w: %s", ErrProviderNotFound, name)
	}
	if !a.runMu.TryLock() {
		return ErrAppRunning
	}
	defer a.runMu.Unlock()
	// A completed job run can be invoked again; release its previous resources first.
	a.mu.Lock()
	previousRun := a.stopped || a.defaultReleaseDone != nil || len(a.registeredCapabilities) > 0 || len(a.runningProviders) > 0
	a.mu.Unlock()
	if previousRun {
		if err := a.Stop(context.Background()); err != nil {
			return err
		}
	}
	defer func() {
		if runErr != nil {
			failedProviders := a.ProviderStatuses()
			runErr = errors.Join(runErr, a.Stop(context.Background()))
			for _, health := range failedProviders {
				if health.Status == StatusFailed {
					if provider, ok := a.Provider(health.Name); ok {
						setObjectStatus(provider, StatusFailed, health.Error)
					}
				}
			}
		}
	}()
	ctx := a.prepareRunContext(ctxs...)
	stopSignals := a.watchSignals(ctx)
	defer stopSignals()
	release, err := a.lockLifecycle(ctx)
	if err != nil {
		return err
	}
	// This defer must run before the earlier rollback defer calls Stop,
	// which acquires the same lifecycle gate on registration failure.
	defer release()

	registeredProviders := a.Providers()
	providers, err := selectProvidersForRun(name, registeredProviders, a.Capabilities(), providerRuntimeCapabilityNameSet(registeredProviders))
	if err != nil {
		a.failRuntimeStatuses(err)
		return err
	}
	if err := a.registerProviderCapabilities(providers); err != nil {
		a.failRuntimeStatuses(err)
		return err
	}

	capabilities, err := SortCapabilities(a.Capabilities())
	if err != nil {
		a.failRuntimeStatuses(err)
		return err
	}
	providers, err = SortProviders(providers, capabilities)
	if err != nil {
		a.failRuntimeStatuses(err)
		return err
	}

	for _, capability := range capabilities {
		if err := ctx.Err(); err != nil {
			return err
		}
		setObjectStatus(capability, StatusBooting, nil)
		if err := capability.Register(a); err != nil {
			setObjectStatus(capability, StatusFailed, err)
			return err
		}
		setObjectStatus(capability, StatusRunning, nil)
		a.addRegisteredCapability(capability)
	}
	for _, provider := range providers {
		if err := ctx.Err(); err != nil {
			return err
		}
		setObjectStatus(provider, StatusBooting, nil)
		if err := provider.Register(a); err != nil {
			setObjectStatus(provider, StatusFailed, err)
			return err
		}
		a.addRunningProvider(provider)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	release()
	return a.startProviders(ctx, providers)
}

func (a *App) Capabilities() []Capability {
	if a == nil {
		return nil
	}
	return a.registryRef().Capabilities()
}

func (a *App) Capability(name string) (Capability, bool) {
	if a == nil || name == "" {
		return nil, false
	}
	return a.registryRef().Capability(name)
}

func (a *App) CapabilitiesByGroup(group string) []Capability {
	if a == nil {
		return nil
	}
	return a.registryRef().CapabilitiesByGroup(group)
}

func (a *App) CapabilitiesByScope(scope string) []Capability {
	if a == nil {
		return nil
	}
	return a.registryRef().CapabilitiesByScope(scope)
}

func (a *App) Providers() []Provider {
	if a == nil {
		return nil
	}
	return a.registryRef().Providers()
}

func (a *App) Provider(name string) (Provider, bool) {
	if a == nil || name == "" {
		return nil, false
	}
	return a.registryRef().Provider(name)
}

func (a *App) ProvidersByGroup(group string) []Provider {
	if a == nil {
		return nil
	}
	return a.registryRef().ProvidersByGroup(group)
}

func (a *App) Commands() []Command {
	if a == nil {
		return nil
	}
	return a.registryRef().Commands()
}

func (a *App) Command(name string) (Command, bool) {
	if a == nil || name == "" {
		return nil, false
	}
	return a.registryRef().Command(name)
}

func (a *App) CommandsByGroup(group string) []Command {
	if a == nil {
		return nil
	}
	return a.registryRef().CommandsByGroup(group)
}

func (a *App) Adapters() []Adapter {
	if a == nil {
		return nil
	}
	return a.registryRef().Adapters()
}

func (a *App) Adapter(name string) (Adapter, bool) {
	if a == nil || name == "" {
		return nil, false
	}
	return a.registryRef().Adapter(name)
}

func (a *App) AdaptersByType(adapterType string) []Adapter {
	if a == nil {
		return nil
	}
	return a.registryRef().AdaptersByType(adapterType)
}

func (a *App) Plugins() []Plugin {
	if a == nil {
		return nil
	}
	return a.registryRef().Plugins()
}

func (a *App) Plugin(name string) (Plugin, bool) {
	if a == nil || name == "" {
		return nil, false
	}
	return a.registryRef().Plugin(name)
}

func (a *App) PluginsByGroup(group string) []Plugin {
	if a == nil {
		return nil
	}
	return a.registryRef().PluginsByGroup(group)
}
