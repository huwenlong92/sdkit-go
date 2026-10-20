package tests

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/huwenlong92/sdkit-go/core/runtime"
)

func TestV1RegisterFailureRollsBackSuccessfulCapabilities(t *testing.T) {
	for _, providerFailure := range []bool{false, true} {
		t.Run(map[bool]string{false: "capability", true: "provider"}[providerFailure], func(t *testing.T) {
			app := runtime.New()
			failure := errors.New("register failed")
			var calls []string
			if err := app.Use(testCapability{name: "resource", register: func(*runtime.App) error { return nil }, shutdown: func(context.Context) error { calls = append(calls, "close"); return nil }}); err != nil {
				t.Fatal(err)
			}
			if providerFailure {
				if err := app.Register(testProvider{name: "service", register: func(*runtime.App) error { return failure }}); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := app.Use(testCapability{name: "broken", register: func(*runtime.App) error { return failure }}); err != nil {
					t.Fatal(err)
				}
			}
			if err := app.Run(); !errors.Is(err, failure) {
				t.Fatalf("Run=%v, want %v", err, failure)
			}
			if !reflect.DeepEqual(calls, []string{"close"}) {
				t.Fatalf("cleanup=%v, want [close]", calls)
			}
			if err := app.Stop(context.Background()); err != nil {
				t.Fatal(err)
			}
			if len(calls) != 1 {
				t.Fatalf("cleanup count=%d, want 1", len(calls))
			}
		})
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
