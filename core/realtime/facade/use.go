package realtime

import (
	"context"
	"errors"

	eventbus "github.com/huwenlong92/sdkit-go/core/eventbus/facade"
	corerealtime "github.com/huwenlong92/sdkit-go/core/realtime"
	"github.com/huwenlong92/sdkit-go/core/runtime"
)

type ConfigLoader func(app *runtime.App) (Config, error)

type UseOption func(*useOptions)

type useOptions struct {
	explicitOnly bool
	config       Config
	configLoader ConfigLoader
	service      Service
	dependencies []runtime.Dependency
	internal     bool
}

func WithConfig(cfg Config) UseOption {
	return func(o *useOptions) {
		o.config = cfg
	}
}

func WithConfigLoader(loader ConfigLoader) UseOption {
	return func(o *useOptions) {
		o.configLoader = loader
	}
}

func WithService(service Service) UseOption {
	return func(o *useOptions) {
		o.service = service
	}
}

func WithDependencies(deps ...runtime.Dependency) UseOption {
	return func(o *useOptions) {
		o.dependencies = append(o.dependencies, deps...)
	}
}

func WithInternal() UseOption {
	return func(o *useOptions) {
		o.internal = true
	}
}

func WithDefault(enabled bool) UseOption { return func(o *useOptions) { o.explicitOnly = !enabled } }

func Use(opts ...UseOption) runtime.Capability {
	o := useOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}

	dependencies := []runtime.Dependency{
		runtime.OptionalBootstrap(),
		runtime.Require(eventbus.Name),
	}
	dependencies = append(dependencies, o.dependencies...)

	var runtimeService Service
	return runtime.NewCapabilityWithMetadataAndDependencies(runtime.CapabilityMetadata{
		Name:        Name,
		Description: "Realtime publisher",
		Group:       runtime.GroupSystem,
		Scope:       runtime.ScopeGlobal,
		Internal:    o.internal,
	}, dependencies, func(app *runtime.App) error {
		config := o.config
		if o.configLoader != nil {
			loaded, err := o.configLoader(app)
			if err != nil {
				return err
			}
			config = loaded
		}

		service := o.service
		if service == nil {
			eventbusService := eventbus.From(app)
			if eventbusService == nil || eventbusService.Bus() == nil {
				return ErrEventBusNotConfigured
			}
			var err error
			service, err = New(config, WithEventBus(eventbusService.Bus()), WithoutDefault())
			if err != nil {
				return err
			}
		}
		if o.service == nil {
			runtimeService = service
		}
		bind := func() error {
			if o.explicitOnly {
				return app.Container().Bind(runtime.Key(Name), service)
			}
			return runtime.BindProcessDefault(app, runtime.Key(Name), service, func() { SetDefault(service); corerealtime.SetDefaultPublisher(service) }, func() { SetDefault(nil); corerealtime.SetDefaultPublisher(nil) })
		}
		if err := bind(); err != nil {
			return errors.Join(err, closeService(&runtimeService))
		}
		return nil
	}, func(context.Context) error {
		return closeService(&runtimeService)
	})
}

func From(app *runtime.App) Service {
	if app != nil {
		if value, ok := app.Container().Get(runtime.Key(Name)); ok {
			if service, ok := value.(Service); ok {
				return service
			}
		}
		return nil
	}
	return Default()
}

func closeService(service *Service) error {
	if service == nil || *service == nil {
		return nil
	}
	current := *service
	*service = nil
	return current.Close()
}
