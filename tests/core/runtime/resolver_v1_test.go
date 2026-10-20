package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/huwenlong92/sdkit-go/core/runtime"
)

func TestV1StrictResolution(t *testing.T) {
	registry := runtime.NewLocalCapabilityRegistry()
	var typedNil *int
	registry.Set("wrong", 42)
	registry.Set("nil", typedNil)
	for _, name := range []string{"wrong", "nil"} {
		if _, err := runtime.ResolveCapability[*int](registry, name); !errors.Is(err, runtime.ErrCapabilityTypeMismatch) {
			t.Fatalf("Resolve(%s)=%v", name, err)
		}
	}
	if _, err := runtime.ResolveCapability[string](registry, "absent"); !errors.Is(err, runtime.ErrCapabilityNotFound) {
		t.Fatalf("missing=%v", err)
	}
	if _, found, err := runtime.OptionalCapability[string](registry, "absent"); found || err != nil {
		t.Fatalf("optional absent=%v,%v", found, err)
	}
	if _, _, err := runtime.OptionalCapability[string](registry, "wrong"); !errors.Is(err, runtime.ErrCapabilityTypeMismatch) {
		t.Fatalf("optional wrong=%v", err)
	}
	ctx := runtime.NewServiceContext[any]("api", "api", nil)
	ctx.Capabilities.Set("value", "global")
	ctx.Capabilities.Set("api.value", "local")
	if value, err := runtime.ResolveCapability[string](ctx, "value"); err != nil || value != "local" {
		t.Fatalf("local=%q,%v", value, err)
	}
	app := runtime.New()
	cause := errors.New("init failed")
	if err := app.Use(runtime.NewCapability("broken", func(*runtime.App) error { return cause })); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(); !errors.Is(err, cause) {
		t.Fatal(err)
	}
	_, err := runtime.ResolveAppCapability[string](app, "broken")
	if !errors.Is(err, cause) || !errors.Is(err, runtime.ErrCapabilityInitialization) {
		t.Fatalf("init error=%v", err)
	}
}

type v1Closer struct{ count int }

func (c *v1Closer) Close() error { c.count++; return nil }
func TestV1BorrowedRegistryDoesNotCloseResource(t *testing.T) {
	registry := runtime.NewLocalCapabilityRegistry()
	owned := &v1Closer{}
	borrowed := &v1Closer{}
	registry.Set("owned", owned)
	registry.Set("owned-alias", owned)
	registry.SetBorrowed("borrowed", borrowed)
	if err := registry.Close(); err != nil {
		t.Fatal(err)
	}
	if err := registry.Close(); err != nil {
		t.Fatal(err)
	}
	if owned.count != 1 || borrowed.count != 0 {
		t.Fatalf("close owned=%d borrowed=%d", owned.count, borrowed.count)
	}
}
func TestV1ConcurrentRunRejected(t *testing.T) {
	app := runtime.New()
	entered := make(chan struct{})
	release := make(chan struct{})
	if err := app.Use(runtime.NewCapability("blocking", func(*runtime.App) error { close(entered); <-release; return nil })); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- app.Run() }()
	<-entered
	if err := app.Run(); !errors.Is(err, runtime.ErrAppRunning) {
		t.Fatalf("concurrent Run=%v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := app.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestV1ServiceCapabilitiesKeepSharedScope(t *testing.T) {
	registry := runtime.NewServiceRegistry[any]()
	registry.RegisterServiceDefinition(runtime.ServiceDefinition[any]{Type: "api", ContextFactory: func(runtime.ServiceContext[any]) (runtime.Service, error) { return nil, nil }, RuntimeCapabilityFactory: func(ctx runtime.RuntimeCapabilityContext[any]) []runtime.CapabilityContract {
		return []runtime.CapabilityContract{runtime.NewCapability("shared", func(*runtime.App) error { return nil }), runtime.NewCapability(ctx.LocalName("local"), func(*runtime.App) error { return nil })}
	}})
	caps := registry.RuntimeCapabilitiesForService(runtime.NewRuntimeCapabilityContext[any]("", "api", "api", "", nil))
	if len(caps) != 2 {
		t.Fatalf("capability count=%d", len(caps))
	}
	if caps[0].Metadata().Scope != runtime.ScopeGlobal || caps[1].Metadata().Scope != runtime.ScopeServiceLocal {
		t.Fatalf("scopes=%s,%s", caps[0].Metadata().Scope, caps[1].Metadata().Scope)
	}
}
