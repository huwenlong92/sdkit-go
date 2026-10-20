package logger

import (
	"context"

	corelogger "github.com/huwenlong92/sdkit-go/core/logger"
	"github.com/huwenlong92/sdkit-go/core/runtime"

	"go.uber.org/zap"
)

type UseOption func(*useOptions)

type useOptions struct {
	explicitOnly bool
	config       Config
	hasConfig    bool
	configLoader ConfigLoader
	logger       *zap.Logger
	dependencies []runtime.Dependency
	internal     bool
}

func defaultUseOptions() useOptions {
	return useOptions{
		dependencies: []runtime.Dependency{
			runtime.OptionalBootstrap(),
		},
		internal: true,
	}
}

func WithConfig(cfg Config) UseOption {
	return func(o *useOptions) {
		o.config = cfg
		o.hasConfig = true
	}
}

func WithLogger(log *zap.Logger) UseOption {
	return func(o *useOptions) {
		o.logger = log
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

func WithInternal() UseOption {
	return func(o *useOptions) {
		o.internal = true
	}
}

func WithExternal() UseOption {
	return func(o *useOptions) {
		o.internal = false
	}
}

// WithDefault(false) keeps the configured or injected logger isolated in its App.
func WithDefault(enabled bool) UseOption { return func(o *useOptions) { o.explicitOnly = !enabled } }

func Use(opts ...UseOption) runtime.Capability {
	o := defaultUseOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}

	var closeOwned func() error
	return runtime.NewCapabilityWithMetadataAndDependencies(runtime.CapabilityMetadata{
		Name:        Name,
		Description: "Zap logger",
		Group:       runtime.GroupSystem,
		Scope:       runtime.ScopeGlobal,
		Internal:    o.internal,
	}, o.dependencies, func(app *runtime.App) error {
		if !o.explicitOnly {
			if err := runtime.BindProcessDefault(app, runtime.Key(Name), true, nil, func() { corelogger.SetManagedDefault(nil) }); err != nil {
				return err
			}
		}
		log := o.logger
		if log == nil {
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
			if !hasConfig && !o.explicitOnly {
				log = corelogger.Default()
			} else {
				created, close, err := corelogger.NewManaged(cfg)
				if err != nil {
					return err
				}
				log = created
				closeOwned = close
			}
		}
		if !o.explicitOnly {
			corelogger.SetManagedDefault(log)
		}
		return app.Container().Bind(runtime.Key(Name), log)
	}, func(context.Context) error {
		if closeOwned == nil {
			return nil
		}
		close := closeOwned
		closeOwned = nil
		return close()
	})
}
