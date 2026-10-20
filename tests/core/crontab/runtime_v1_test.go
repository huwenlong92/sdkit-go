package crontab_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/huwenlong92/sdkit-go/core/crontab"
)

type v1Store struct {
	crontab.NoopStore
	finishErr error
	finished  bool
}

func (s *v1Store) MarkFinished(ctx context.Context, _ crontab.RunLog) error {
	s.finished = true
	s.finishErr = ctx.Err()
	return s.finishErr
}

type v1Lock struct {
	mu             sync.Mutex
	held           bool
	unlocked       bool
	unlockErr      error
	releaseFailure error
	releaseCount   int
}

func (l *v1Lock) Acquire(ctx context.Context, key string, ttl time.Duration) (crontab.Lock, bool, error) {
	return l.TryLock(ctx, key, ttl)
}
func (l *v1Lock) TryLock(context.Context, string, time.Duration) (crontab.Lock, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held {
		return nil, false, nil
	}
	l.held = true
	return l, true, nil
}
func (l *v1Lock) Release(ctx context.Context) error { return l.Unlock(ctx) }
func (l *v1Lock) Unlock(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.held = false
	l.unlocked = true
	l.releaseCount++
	l.unlockErr = ctx.Err()
	return errors.Join(l.unlockErr, l.releaseFailure)
}
func (l *v1Lock) Refresh(context.Context, time.Duration) error { return nil }
func TestV1CrontabCancellationUnlocksWithLiveContext(t *testing.T) {
	registry := crontab.NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handler := func(ctx context.Context, _ crontab.Job) error { cancel(); return ctx.Err() }
	if err := registry.Register(crontab.Template{Key: "cancel", Enabled: true, AllowDB: true, Handler: crontab.RunHandlerFromFunc(handler)}); err != nil {
		t.Fatal(err)
	}
	lock := &v1Lock{}
	store := &v1Store{}
	cfg := crontab.DefaultConfig()
	runner := crontab.NewRunner(crontab.RunnerOptions{Config: cfg, Registry: registry, Locker: lock, Store: store})
	result := runner.RunResult(ctx, crontab.Job{ID: "v1", Name: "cancel", Enabled: true})
	if !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("result=%+v", result)
	}
	if !store.finished || store.finishErr != nil {
		t.Fatalf("terminal log finished=%v err=%v", store.finished, store.finishErr)
	}
	if !lock.unlocked || lock.unlockErr != nil {
		t.Fatalf("unlocked=%v err=%v", lock.unlocked, lock.unlockErr)
	}
}
func TestV1CrontabOverlapTimeoutAndRuntime(t *testing.T) {
	registry := crontab.NewRegistry()
	entered := make(chan struct{})
	finished := make(chan struct{})
	if err := registry.Register(crontab.Template{Key: "slow", Enabled: true, AllowDB: true, Timeout: 25 * time.Millisecond, Handler: crontab.RunHandlerFromFunc(func(ctx context.Context, _ crontab.Job) error {
		close(entered)
		<-ctx.Done()
		close(finished)
		return ctx.Err()
	})}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(crontab.Template{Key: "slow"}); err == nil {
		t.Fatal("duplicate template accepted")
	}
	lock := &v1Lock{}
	state := crontab.NewRuntimeState()
	runner := crontab.NewRunner(crontab.RunnerOptions{Config: crontab.DefaultConfig(), Registry: registry, Locker: lock, Runtime: state})
	job := crontab.Job{ID: "v1", Name: "slow", Enabled: true}
	done := make(chan crontab.RunResult, 1)
	go func() { done <- runner.RunResult(context.Background(), job) }()
	<-entered
	if result := runner.RunResult(context.Background(), job); result.Status != crontab.StatusLocked {
		t.Fatalf("overlap=%+v", result)
	}
	result := <-done
	if result.Status != crontab.StatusTimeout {
		t.Fatalf("timeout=%+v", result)
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("handler did not exit")
	}
	if info, ok := state.Get("v1"); !ok || info.Status != crontab.RuntimeFailed {
		t.Fatalf("runtime=%+v found=%v", info, ok)
	}
}

func TestV1CrontabUnlockErrorRetainsExecutionResult(t *testing.T) {
	for _, handlerFails := range []bool{false, true} {
		t.Run(fmt.Sprintf("handlerFails=%t", handlerFails), func(t *testing.T) {
			handlerErr, unlockErr := errors.New("handler failed"), errors.New("unlock failed")
			registry := crontab.NewRegistry()
			if err := registry.Register(crontab.Template{Key: "cleanup", Enabled: true, AllowDB: true, Handler: crontab.RunHandlerFromFunc(func(context.Context, crontab.Job) error {
				if handlerFails {
					return handlerErr
				}
				return nil
			})}); err != nil {
				t.Fatal(err)
			}
			lock := &v1Lock{releaseFailure: unlockErr}
			store := &v1Store{}
			state := crontab.NewRuntimeState()
			runner := crontab.NewRunner(crontab.RunnerOptions{Config: crontab.DefaultConfig(), Registry: registry, Locker: lock, Store: store, Runtime: state})
			result := runner.RunResult(context.Background(), crontab.Job{ID: "cleanup", Name: "cleanup", Enabled: true})
			if result.Status != crontab.StatusFailed || !errors.Is(result.Err, unlockErr) || errors.Is(result.Err, handlerErr) != handlerFails {
				t.Fatalf("result=%+v", result)
			}
			if lock.releaseCount != 1 || lock.unlockErr != nil {
				t.Fatalf("cleanup count=%d context=%v", lock.releaseCount, lock.unlockErr)
			}
			if !store.finished || store.finishErr != nil {
				t.Fatalf("terminal write=%v error=%v", store.finished, store.finishErr)
			}
			if info, ok := state.Get("cleanup"); !ok || info.Status != crontab.RuntimeFailed {
				t.Fatalf("runtime=%+v found=%v", info, ok)
			}
		})
	}
}
