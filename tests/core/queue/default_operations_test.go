package queue_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huwenlong92/sdkit-go/core/queue"
	"github.com/huwenlong92/sdkit-go/core/runtime"
)

type blockingOperationsManager struct {
	queue.Manager
	entered chan struct{}
	release chan struct{}
}

func (m *blockingOperationsManager) ListQueues(context.Context) ([]*queue.QueueInfo, error) {
	close(m.entered)
	<-m.release
	return []*queue.QueueInfo{{Name: "contract"}}, nil
}

func TestDefaultOperationsWaitForInFlightCallAndUnbind(t *testing.T) {
	if _, err := queue.ListQueues(context.Background()); !errors.Is(err, queue.ErrNotInitialized) {
		t.Fatalf("before bind = %v", err)
	}
	app := runtime.New()
	other := runtime.New()
	t.Cleanup(func() { _ = app.Stop(context.Background()); _ = other.Stop(context.Background()) })
	manager := &blockingOperationsManager{entered: make(chan struct{}), release: make(chan struct{})}
	instance := queue.NewRuntimeInstanceFromParts(queue.RuntimeParts{Manager: manager})
	if err := queue.BindDefaultOperations(app, instance); err != nil {
		t.Fatal(err)
	}
	if err := queue.BindDefaultOperations(other, instance); !errors.Is(err, runtime.ErrDefaultConflict) {
		t.Fatalf("conflict = %v", err)
	}
	done := make(chan error, 1)
	go func() {
		items, err := queue.ListQueues(context.Background())
		if err == nil && (len(items) != 1 || items[0].Name != "contract") {
			err = errors.New("unexpected queue result")
		}
		done <- err
	}()
	<-manager.entered
	stopped := make(chan error, 1)
	go func() { stopped <- app.Stop(context.Background()) }()
	select {
	case err := <-stopped:
		t.Fatalf("stopped during active call: %v", err)
	default:
	}
	close(manager.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
	if _, err := queue.ListQueues(context.Background()); !errors.Is(err, queue.ErrNotInitialized) {
		t.Fatalf("after stop = %v", err)
	}
}

type countedClient struct {
	queue.Client
	closes atomic.Int32
}

func (c *countedClient) Close() error { c.closes.Add(1); return nil }
func TestRuntimeInstanceConcurrentCloseIsIdempotent(t *testing.T) {
	client := &countedClient{}
	instance := queue.NewRuntimeInstanceFromParts(queue.RuntimeParts{Client: client})
	var group sync.WaitGroup
	for range 16 {
		group.Go(func() {
			if err := instance.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	if client.closes.Load() != 1 {
		t.Fatalf("close count = %d, want 1", client.closes.Load())
	}
}

func TestDefaultOperationsShutdownDeadlineCanBeRetried(t *testing.T) {
	app := runtime.New()
	t.Cleanup(func() { _ = app.Stop(context.Background()) })
	manager := &blockingOperationsManager{entered: make(chan struct{}), release: make(chan struct{})}
	if err := queue.BindDefaultOperations(app, queue.NewRuntimeInstanceFromParts(queue.RuntimeParts{Manager: manager})); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := queue.ListQueues(context.Background()); done <- err }()
	<-manager.entered
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	err := app.Stop(ctx)
	close(manager.release)
	if callErr := <-done; callErr != nil {
		t.Fatal(callErr)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stop = %v, want deadline exceeded", err)
	}
	if err := app.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := queue.ListQueues(context.Background()); !errors.Is(err, queue.ErrNotInitialized) {
		t.Fatalf("after retry = %v", err)
	}
}
