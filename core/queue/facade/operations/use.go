package operations

import (
	"context"
	"errors"

	"github.com/huwenlong92/sdkit-go/core/queue"
	redis "github.com/huwenlong92/sdkit-go/core/redis/facade"
	"github.com/huwenlong92/sdkit-go/core/runtime"
	queuedriver "github.com/huwenlong92/sdkit-go/pkg/queue/driver"
)

var ErrOperationsConfigRequired = errors.New("queue operations facade: config required")

type UseOption func(*useOptions)

type useOptions struct {
	bindDefault          bool
	runtimeOptionsLoader func(*runtime.App) ([]queue.RuntimeInstanceOption, error)
	name                 runtime.Key
	config               Config
	hasConfig            bool
	configLoader         ConfigLoader
	dependencies         []runtime.Dependency
	runtimeOpts          []queue.RuntimeInstanceOption
	internal             bool
}

func WithName(name string) UseOption {
	return func(o *useOptions) {
		o.name = runtime.Key(name)
	}
}

func WithConfig(cfg Config) UseOption {
	return func(o *useOptions) {
		o.config = cfg
		o.hasConfig = true
	}
}

func WithConfigLoader(loader ConfigLoader) UseOption {
	return func(o *useOptions) {
		o.configLoader = loader
	}
}

func WithDependencies(deps ...runtime.Dependency) UseOption {
	return func(o *useOptions) {
		o.dependencies = append(o.dependencies, deps...)
	}
}

func WithRuntimeOptions(opts ...queue.RuntimeInstanceOption) UseOption {
	return func(o *useOptions) {
		o.runtimeOpts = append(o.runtimeOpts, opts...)
	}
}

func WithInternal() UseOption {
	return func(o *useOptions) {
		o.internal = true
	}
}

// WithDefault opts into the process operations API; isolated instances keep it disabled.
func WithDefault(enabled bool) UseOption { return func(o *useOptions) { o.bindDefault = enabled } }

// WithRuntimeOptionsLoader resolves dependencies after capability initialization.
func WithRuntimeOptionsLoader(loader func(*runtime.App) ([]queue.RuntimeInstanceOption, error)) UseOption {
	return func(o *useOptions) { o.runtimeOptionsLoader = loader }
}

func Use(opts ...UseOption) runtime.Capability {
	o := useOptions{name: runtime.Key(Name)}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	if o.name == "" {
		o.name = runtime.Key(Name)
	}

	dependencies := []runtime.Dependency{
		runtime.OptionalBootstrap(),
		runtime.Optional(redis.Name),
	}
	dependencies = append(dependencies, o.dependencies...)

	var registered *queue.RuntimeInstance
	return runtime.NewCapabilityWithMetadataAndDependencies(runtime.CapabilityMetadata{
		Name:        string(o.name),
		Description: "Queue operations",
		Group:       runtime.GroupSystem,
		Scope:       runtime.ScopeGlobal,
		Internal:    o.internal,
	}, dependencies, func(app *runtime.App) error {
		cfg := o.config
		hasConfig := o.hasConfig
		if o.configLoader != nil {
			loaded, err := o.configLoader(app)
			if err != nil {
				return err
			}
			cfg = loaded
			hasConfig = true
		}
		if !hasConfig {
			return ErrOperationsConfigRequired
		}
		runtimeOpts := append([]queue.RuntimeInstanceOption(nil), o.runtimeOpts...)
		if o.runtimeOptionsLoader != nil {
			loaded, err := o.runtimeOptionsLoader(app)
			if err != nil {
				return err
			}
			runtimeOpts = append(runtimeOpts, loaded...)
		}
		queueCfg := cfg.Queue
		if err := queuedriver.Register(); err != nil {
			return err
		}
		client, err := queue.NewClient(queueCfg)
		if err != nil {
			return err
		}
		manager, err := queue.NewManager(queueCfg)
		if err != nil {
			_ = client.Close()
			return err
		}
		metadata := cfg.Metadata
		if isZeroMetadata(metadata) {
			metadata = queue.RuntimeMetadataFromConfig("", "", queueCfg)
		}
		instanceOpts := []queue.RuntimeInstanceOption{queue.WithRuntimeMetadata(metadata)}
		instanceOpts = append(instanceOpts, runtimeOpts...)
		registered = queue.NewRuntimeInstanceFromParts(queue.RuntimeParts{Client: client, Manager: manager}, instanceOpts...)
		if err := app.Container().Bind(o.name, registered); err != nil {
			return errors.Join(err, registered.Close())
		}
		if o.bindDefault {
			if err := queue.BindDefaultOperations(app, registered); err != nil {
				closeErr := registered.Close()
				registered = nil
				return errors.Join(err, closeErr)
			}
		}
		return nil
	}, func(context.Context) error {
		if registered == nil {
			return nil
		}
		current := registered
		registered = nil
		return current.Close()
	})
}

func isZeroMetadata(metadata queue.RuntimeMetadata) bool {
	return metadata.Name == "" &&
		metadata.Service == "" &&
		metadata.Driver == "" &&
		len(metadata.Queues) == 0 &&
		metadata.DefaultQueue == "" &&
		metadata.Concurrency == 0
}
