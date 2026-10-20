package tests

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huwenlong92/sdkit-go/core/runtime"
)

func TestStopDefaultTimeoutRemainsStoppingAndCanRetry(t *testing.T) {
	app, other := runtime.New(), runtime.New()
	release, cleared := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(func() { unblock(); _ = app.Stop(context.Background()); _ = other.Stop(context.Background()) })
	var shutdowns, clears atomic.Int32
	const key runtime.Key = "stop-timeout-default"
	if err := app.Use(runtime.NewCapabilityWithShutdown("owned", func(app *runtime.App) error {
		return runtime.BindProcessDefault(app, key, true, nil, func() { <-release; clears.Add(1); close(cleared) })
	}, func(context.Context) error { shutdowns.Add(1); return nil })); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if err := app.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("first Stop = %v", err)
	}
	health := app.CapabilityStatus("owned")
	if health.Status != runtime.StatusStopping || health.Ready {
		t.Errorf("pending shutdown health = %+v", health)
	}
	for _, health := range app.CapabilityStatuses() {
		if health.Status != runtime.StatusStopping || health.Ready {
			t.Errorf("pending shutdown list = %+v", health)
		}
	}
	if shutdowns.Load() != 0 {
		t.Fatal("capability closed before default calls drained")
	}
	unblock()
	select {
	case <-cleared:
	case <-time.After(time.Second):
		t.Fatal("default clear did not finish")
	}
	if err := runtime.BindProcessDefault(other, key, true, nil, nil); !errors.Is(err, runtime.ErrDefaultConflict) {
		t.Fatalf("reservation before retry = %v", err)
	}
	if err := runtime.BindProcessDefault(app, "new-after-timeout", true, nil, nil); !errors.Is(err, runtime.ErrAppStopped) {
		t.Fatalf("bind after timeout = %v", err)
	}
	// Multiple retry callers must serialize, observe completion, and never repeat Shutdown.
	results := make(chan error, 4)
	for range 4 {
		go func() { results <- app.Stop(context.Background()) }()
	}
	for range 4 {
		select {
		case err := <-results:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("retry Stop deadlocked")
		}
	}
	if shutdowns.Load() != 1 || clears.Load() != 1 {
		t.Fatalf("Shutdown/clear = %d/%d", shutdowns.Load(), clears.Load())
	}
	if health := app.CapabilityStatus("owned"); health.Status != runtime.StatusStopped || health.Ready {
		t.Fatalf("final health = %+v", health)
	}
	if _, ok := app.Container().Get(key); ok {
		t.Fatal("stopped container retains default")
	}
	if err := runtime.BindProcessDefault(other, key, true, nil, nil); err != nil {
		t.Fatalf("final reservation not released: %v", err)
	}
}

func TestStopDefaultTimeoutConcurrentRun(t *testing.T) {
	for _, mode := range []string{"Run", "RunProvider", "Run/service", "RunProvider/service"} {
		t.Run(mode, func(t *testing.T) {
			app := runtime.New()
			started, finishStart, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var startOnce, releaseOnce sync.Once
			finish := func() { startOnce.Do(func() { close(finishStart) }) }
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(func() { finish(); unblock(); _ = app.Stop(context.Background()) })
			var shutdowns atomic.Int32
			if err := app.Use(runtime.NewCapabilityWithShutdown("owned", func(app *runtime.App) error {
				return runtime.BindProcessDefault(app, "concurrent-stop-default", true, nil, func() { <-release })
			}, func(context.Context) error { shutdowns.Add(1); return nil })); err != nil {
				t.Fatal(err)
			}
			metadata := runtime.ProviderMetadata{}
			if mode == "Run/service" || mode == "RunProvider/service" {
				metadata.Mode = runtime.ProviderModeService
			}
			if err := app.Register(testProvider{name: "job", metadata: metadata, start: func(ctx context.Context) error { close(started); <-finishStart; return ctx.Err() }}); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				if mode == "Run" || mode == "Run/service" {
					done <- app.Run()
				} else {
					done <- app.RunProvider("job")
				}
			}()
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("provider did not start")
			}
			if err := app.Run(); !errors.Is(err, runtime.ErrAppRunning) {
				t.Fatalf("concurrent Run = %v", err)
			}
			if err := app.RunProvider("job"); !errors.Is(err, runtime.ErrAppRunning) {
				t.Fatalf("concurrent RunProvider = %v", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
			defer cancel()
			if err := app.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("first Stop = %v", err)
			}
			finish()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("intentional shutdown marked run failed: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("Run blocked on automatic Stop retry")
			}
			if health := app.ProviderStatus("job"); health.Status != runtime.StatusStopped || health.Ready {
				t.Errorf("provider health = %+v", health)
			}
			if shutdowns.Load() != 0 {
				t.Fatal("capability shut down while default still draining")
			}
			unblock()
			if err := app.Stop(context.Background()); err != nil {
				t.Fatal(err)
			}
			if shutdowns.Load() != 1 {
				t.Fatalf("Shutdown calls = %d", shutdowns.Load())
			}
		})
	}
}
