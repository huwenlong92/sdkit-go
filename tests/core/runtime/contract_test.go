package tests

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/huwenlong92/sdkit-go/core/runtime"
)

func TestProcessDefaultConflictRollbackAndRebind(t *testing.T) {
	first, second := runtime.New(), runtime.New()
	t.Cleanup(func() { _ = first.Stop(context.Background()); _ = second.Stop(context.Background()) })
	published, cleared := 0, 0
	bind := func(app *runtime.App) error {
		return runtime.BindProcessDefault(app, "contract.default", new(int), func() { published++ }, func() { cleared++ })
	}
	if err := bind(first); err != nil {
		t.Fatal(err)
	}
	if err := bind(second); !errors.Is(err, runtime.ErrDefaultConflict) {
		t.Fatalf("conflict = %v, want %v", err, runtime.ErrDefaultConflict)
	}
	if err := second.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if cleared != 0 {
		t.Fatalf("non-owner cleared = %d, want 0", cleared)
	}
	if err := first.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	third := runtime.New()
	t.Cleanup(func() { _ = third.Stop(context.Background()) })
	if err := bind(third); err != nil {
		t.Fatal(err)
	}
	if published != 2 || cleared != 1 {
		t.Fatalf("published/cleared = %d/%d, want 2/1", published, cleared)
	}
}

type contractService struct {
	name    string
	started chan<- string
}

func (s contractService) ServiceInfo() runtime.ServiceInfo {
	return runtime.ServiceInfo{Name: s.name, Enabled: true}
}
func (s contractService) Start(ctx context.Context) error {
	s.started <- s.name
	<-ctx.Done()
	return nil
}
func (s contractService) Shutdown(context.Context) error { return nil }

func TestRunnerFactoryRunAndServeIsolation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		services []string
	}{
		{name: "run", services: []string{"one"}},
		{name: "serve", services: []string{"one", "two"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var created atomic.Int32
			var declarations atomic.Int32
			started := make(chan string, 2)
			shared := new(int)
			local := make(map[any]bool)
			runner := runtime.NewServiceRunner(runtime.ServiceRunnerOptions[struct{}]{
				LoadSpecs: func(string) (map[string]runtime.ServiceSpec, error) {
					return map[string]runtime.ServiceSpec{"one": {Type: "test"}, "two": {Type: "test"}}, nil
				},
				CapabilityFactories: map[string]runtime.CapabilityFactory{
					"shared": func() runtime.CapabilityContract {
						created.Add(1)
						return runtime.NewCapability("shared", func(app *runtime.App) error { return app.Container().Bind("shared", shared) })
					},
				},
				Providers: []runtime.ServiceProvider[struct{}]{runtime.ServiceProviderFunc[struct{}](func(app *runtime.ServiceApp[struct{}]) error {
					app.Service("test").Kind(runtime.ServiceKindCLI).RequireCapabilities("shared").RuntimeCapabilities(func(ctx runtime.RuntimeCapabilityContext[struct{}]) []runtime.CapabilityContract {
						declarations.Add(1)
						return []runtime.CapabilityContract{runtime.NewCapabilityWithMetadataAndDependencies(runtime.CapabilityMetadata{Name: ctx.LocalName("local"), Scope: runtime.ScopeServiceLocal}, []runtime.Dependency{runtime.Require("shared")}, func(app *runtime.App) error {
							return app.Container().Bind(runtime.Key(ctx.LocalName("local")), new(int))
						}, nil)}
					}).FactoryContext(func(ctx runtime.ServiceContext[struct{}]) (runtime.Service, error) {
						got, err := runtime.ResolveCapability[*int](&ctx, "shared")
						if err != nil {
							return nil, err
						}
						if got != shared {
							return nil, errors.New("shared instance differs")
						}
						value, err := runtime.ResolveCapability[*int](&ctx, "local")
						if err != nil {
							return nil, err
						}
						if local[value] {
							return nil, errors.New("local instance reused")
						}
						local[value] = true
						return contractService{name: ctx.Name, started: started}, nil
					})
					return nil
				})},
			})
			app, err := runner.NewApp(runtime.ServiceRunOptions{Services: tc.services})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = app.Stop(context.Background()) })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				if len(tc.services) == 1 {
					done <- runtime.RunProvider(ctx, app, "one")
				} else {
					done <- runtime.RunAllProviders(ctx, app)
				}
			}()
			for range tc.services {
				select {
				case <-started:
				case err := <-done:
					t.Fatalf("exited before services started: %v", err)
				}
			}
			cancel()
			err = <-done
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if created.Load() != 1 || declarations.Load() != int32(len(tc.services)) {
				t.Fatalf("factory/declarations = %d/%d, want 1/%d", created.Load(), declarations.Load(), len(tc.services))
			}
		})
	}
}

func TestDefaultReservationSurvivesCapabilityShutdown(t *testing.T) {
	app, other := runtime.New(), runtime.New()
	t.Cleanup(func() { _ = app.Stop(context.Background()); _ = other.Stop(context.Background()) })
	entered, release := make(chan struct{}), make(chan struct{})
	cap := runtime.NewCapabilityWithShutdown("contract", func(app *runtime.App) error { return runtime.BindProcessDefault(app, "reserved", true, nil, nil) }, func(context.Context) error { close(entered); <-release; return nil })
	if err := app.Use(cap); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- app.Stop(context.Background()) }()
	<-entered
	err := runtime.BindProcessDefault(other, "reserved", true, nil, nil)
	close(release)
	if stopErr := <-done; stopErr != nil {
		t.Fatal(stopErr)
	}
	if !errors.Is(err, runtime.ErrDefaultConflict) {
		t.Fatalf("during shutdown = %v, want conflict", err)
	}
	if err := runtime.BindProcessDefault(other, "reserved", true, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := app.Container().Get("reserved"); ok {
		t.Fatal("stopped container retains resource")
	}
}

func TestCapabilityCannotBeSharedAcrossApps(t *testing.T) {
	first, second := runtime.New(), runtime.New()
	t.Cleanup(func() { _ = first.Stop(context.Background()); _ = second.Stop(context.Background()) })
	var constructed, closed atomic.Int32
	cap := runtime.NewCapabilityWithShutdown("owned", func(*runtime.App) error { constructed.Add(1); return nil }, func(context.Context) error { closed.Add(1); return nil })
	if err := first.Use(cap); err != nil {
		t.Fatal(err)
	}
	if err := second.Use(cap); err != nil {
		t.Fatal(err)
	}
	if err := first.Run(); err != nil {
		t.Fatal(err)
	}
	if err := second.Run(); !errors.Is(err, runtime.ErrCapabilityInUse) {
		t.Fatalf("second run = %v, want in use", err)
	}
	if closed.Load() != 0 {
		t.Fatalf("second app closed owner's resource %d times", closed.Load())
	}
	if status := first.CapabilityStatus("owned").Status; status != runtime.StatusRunning {
		t.Fatalf("owner status = %s, want running", status)
	}
	if err := first.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if constructed.Load() != 1 || closed.Load() != 1 {
		t.Fatalf("constructed/closed = %d/%d, want 1/1", constructed.Load(), closed.Load())
	}
}
