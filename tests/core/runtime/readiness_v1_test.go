package tests

import (
	"context"
	"testing"

	"github.com/huwenlong92/sdkit-go/core/runtime"
)

type v1ReadyProvider struct {
	testProvider
	ready chan struct{}
}

func (p v1ReadyProvider) Ready() <-chan struct{} { return p.ready }
func TestV1RunningDoesNotImplyReady(t *testing.T) {
	app := runtime.New()
	entered := make(chan struct{})
	ready := make(chan struct{})
	p := v1ReadyProvider{testProvider: testProvider{name: "service", metadata: runtime.ProviderMetadata{Mode: runtime.ProviderModeService}, start: func(ctx context.Context) error { close(entered); <-ctx.Done(); return nil }}, ready: ready}
	if err := app.Register(p); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- app.Run() }()
	<-entered
	if app.ProviderStatus("service").Ready {
		t.Fatal("service ready before signal")
	}
	close(ready)
	if !app.ProviderStatus("service").Ready {
		t.Fatal("service not ready after signal")
	}
	if err := app.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if app.ProviderStatus("service").Ready {
		t.Fatal("stopped service ready")
	}
}
