package tests

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huwenlong92/sdkit-go/core/runtime"
)

func TestV1RegisterFailureRollsBackSuccessfulCapabilities(t *testing.T) {
	for _, single := range []bool{false, true} {
		for _, providerFailure := range []bool{false, true} {
			for _, cleanupFails := range []bool{false, true} {
				name := fmt.Sprintf("single=%t/provider=%t/cleanupError=%t", single, providerFailure, cleanupFails)
				t.Run(name, func(t *testing.T) {
					app := runtime.New()
					failure := errors.New("register failed")
					closeFailure := errors.New("close failed")
					var closes atomic.Int32
					cleanup := func(context.Context) error {
						closes.Add(1)
						if cleanupFails {
							return closeFailure
						}
						return nil
					}
					if err := app.Use(testCapability{name: "resource", register: func(*runtime.App) error { return nil }, shutdown: cleanup}); err != nil {
						t.Fatal(err)
					}
					provider := testProvider{name: "service", dependencies: []runtime.Dependency{runtime.Require("resource")}}
					if providerFailure {
						provider.register = func(*runtime.App) error { return failure }
					} else {
						if err := app.Use(testCapability{name: "broken", dependencies: []runtime.Dependency{runtime.Require("resource")}, register: func(*runtime.App) error { return failure }}); err != nil {
							t.Fatal(err)
						}
						provider.dependencies = append(provider.dependencies, runtime.Require("broken"))
					}
					if err := app.Register(provider); err != nil {
						t.Fatal(err)
					}
					done := make(chan error, 1)
					started := time.Now()
					go func() {
						if single {
							done <- app.RunProvider("service")
						} else {
							done <- app.RunAllProviders()
						}
					}()
					var runErr error
					select {
					case runErr = <-done:
					case <-time.After(2 * time.Second):
						t.Fatal("rollback waited for lifecycle gate; must finish before default 10s Stop timeout")
					}
					if !errors.Is(runErr, failure) || errors.Is(runErr, context.DeadlineExceeded) {
						t.Fatalf("Run=%v", runErr)
					}
					if errors.Is(runErr, closeFailure) != cleanupFails {
						t.Fatalf("cleanup error lost: %v", runErr)
					}
					if closes.Load() != 1 {
						t.Fatalf("close count=%d, want 1", closes.Load())
					}
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					defer cancel()
					if err := app.Stop(ctx); errors.Is(err, closeFailure) != cleanupFails || errors.Is(err, context.DeadlineExceeded) {
						t.Fatalf("Stop=%v", err)
					}
					if closes.Load() != 1 {
						t.Fatalf("repeat Stop closed %d times", closes.Load())
					}
					t.Logf("rollback and repeated Stop completed in %s", time.Since(started))
				})
			}
		}
	}
}

func TestV1StopRetainsShutdownError(t *testing.T) {
	app := runtime.New()
	failure := errors.New("close failed")
	if err := app.Use(runtime.NewCapabilityWithShutdown("resource", func(*runtime.App) error { return nil }, func(context.Context) error { return failure })); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := app.Stop(context.Background()); !errors.Is(err, failure) {
			t.Fatalf("Stop %d=%v", i, err)
		}
	}
	if app.CapabilityStatus("resource").Ready {
		t.Fatal("failed resource ready")
	}
}

func TestV1FirstRunPreservesCallerAppContext(t *testing.T) {
	app := runtime.New()
	parent := app.Context()
	if err := app.Use(runtime.NewCapability("resource", func(app *runtime.App) error { return app.Context().Err() })); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(parent); err != nil {
		t.Fatal(err)
	}
	if parent.Err() != nil {
		t.Fatalf("caller context canceled: %v", parent.Err())
	}
	if err := app.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestV1StopDuringRegistrationCancelsAndRollsBack(t *testing.T) {
	app := runtime.New()
	entered := make(chan struct{})
	closed := make(chan struct{})
	if err := app.Use(runtime.NewCapabilityWithShutdown("resource", func(app *runtime.App) error { close(entered); <-app.Context().Done(); return nil }, func(context.Context) error { close(closed); return nil })); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- app.Run() }()
	<-entered
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := app.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run=%v", err)
	}
	select {
	case <-closed:
	default:
		t.Fatal("registered resource leaked")
	}
}
