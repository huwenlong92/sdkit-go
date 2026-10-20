package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrDefaultConflict means another App already owns a process-wide default.
var ErrDefaultConflict = errors.New("runtime: process default already bound")
var ErrAppStopped = errors.New("runtime: app is stopping or stopped")

var processDefaults = struct {
	sync.Mutex
	owners map[Key]*App
}{owners: make(map[Key]*App)}

// BindProcessDefault publishes a legacy default only after a successful container
// bind. The App releases it before shutting down its capabilities. Explicit-only
// capabilities should bind their container directly instead.
func BindProcessDefault(app *App, key Key, value any, publish, clear func()) error {
	if app == nil {
		return ErrAppNil
	}
	processDefaults.Lock()
	defer processDefaults.Unlock()
	if _, exists := processDefaults.owners[key]; exists {
		return fmt.Errorf("%w: %s", ErrDefaultConflict, key)
	}
	container := app.Container()
	app.mu.Lock()
	defer app.mu.Unlock()
	if app.stopping || app.stopped || app.defaultReleaseDone != nil {
		return ErrAppStopped
	}
	if err := container.Bind(key, value); err != nil {
		return err
	}
	processDefaults.owners[key] = app
	if publish != nil {
		publish()
	}
	app.defaultKeys = append(app.defaultKeys, key)
	app.defaultReleases = append(app.defaultReleases, func() {
		processDefaults.Lock()
		defer processDefaults.Unlock()
		if processDefaults.owners[key] == app {
			if clear != nil {
				clear()
			}
		}
	})
	return nil
}

// releaseProcessDefaults honors Stop's deadline without closing resources still
// used by a package call. A subsequent Stop can finish after those calls exit.
func (a *App) releaseProcessDefaults(ctx context.Context) error {
	a.mu.Lock()
	done := a.defaultReleaseDone
	if done == nil {
		done = make(chan struct{})
		a.defaultReleaseDone = done
		releases := a.defaultReleases
		a.defaultReleases = nil
		go func() {
			defer close(done)
			for i := len(releases) - 1; i >= 0; i-- {
				releases[i]()
			}
		}()
	}
	a.mu.Unlock()
	select {
	case <-done:
		return ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *App) releaseDefaultReservations() {
	a.mu.Lock()
	keys := a.defaultKeys
	a.defaultKeys = nil
	a.mu.Unlock()
	processDefaults.Lock()
	defer processDefaults.Unlock()
	for _, key := range keys {
		if processDefaults.owners[key] == a {
			delete(processDefaults.owners, key)
		}
	}
}
