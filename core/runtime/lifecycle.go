package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const DefaultStopTimeout = 10 * time.Second

func (a *App) Stop(ctx context.Context) error {
	if a == nil {
		return ErrAppNil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := withDefaultStopTimeout(ctx)
	defer cancel()
	a.cancelRuntimeContext()
	release, err := a.lockLifecycle(ctx)
	if err != nil {
		return err
	}
	defer release()

	a.mu.Lock()
	if a.stopped {
		err := a.stopErr
		a.mu.Unlock()
		return err
	}
	a.stopping = true
	providers := append([]Provider(nil), a.runningProviders...)
	capabilities := append([]Capability(nil), a.registeredCapabilities...)
	runtimeCancel := a.cancel
	a.mu.Unlock()

	if runtimeCancel != nil {
		runtimeCancel()
	}
	providerErr := a.stopProviders(ctx, providers, "")
	if defaultErr := a.releaseProcessDefaults(ctx); defaultErr != nil {
		a.mu.Lock()
		a.stopping = false
		a.mu.Unlock()
		return errors.Join(providerErr, defaultErr)
	}
	err = errors.Join(providerErr, a.shutdownCapabilities(ctx, capabilities))
	a.Container().Clear()
	// A timed-out shutdown may still be running. Keep its default reservations
	// so a late callback cannot tear down a new App's process state.
	if ctx.Err() == nil {
		a.releaseDefaultReservations()
	}

	a.mu.Lock()
	a.runningProviders = nil
	a.registeredCapabilities = nil
	a.stopped = true
	a.stopErr = err
	a.stopping = false
	a.mu.Unlock()

	return err
}

func (a *App) prepareRunContext(ctxs ...context.Context) context.Context {
	parent := context.Background()
	if len(ctxs) > 0 && ctxs[0] != nil {
		parent = ctxs[0]
	}
	ctx, cancel := context.WithCancel(parent)

	a.mu.Lock()
	a.ctx = ctx
	a.cancel = cancel
	a.registeredCapabilities = nil
	a.defaultReleaseDone = nil
	a.runningProviders = nil
	a.stopping = false
	a.stopped = false
	a.stopErr = nil
	a.mu.Unlock()
	a.resetRuntimeStatuses()

	return ctx
}

func (a *App) addRegisteredCapability(capability Capability) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.registeredCapabilities = append(a.registeredCapabilities, capability)
}

func (a *App) addRunningProvider(provider Provider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.runningProviders = append(a.runningProviders, provider)
}

func (a *App) clearRunningProviders() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.runningProviders = nil
}

func (a *App) cancelRuntimeContext() {
	a.mu.Lock()
	cancel := a.cancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) isStopping() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stopping || a.stopped
}

type providerStartResult struct {
	provider Provider
	err      error
}

func (a *App) startProviders(ctx context.Context, providers []Provider) error {
	serviceResults := make(chan providerStartResult, len(providers))
	serviceCount := 0

	for _, provider := range providers {
		if err := ctx.Err(); err != nil {
			return err
		}
		setObjectStatus(provider, StatusBooting, nil)

		if providerMode(provider) == ProviderModeService {
			serviceCount++
			go func(provider Provider) {
				serviceResults <- providerStartResult{
					provider: provider,
					err:      provider.Start(ctx),
				}
			}(provider)
			setObjectStatus(provider, StatusRunning, nil)
			continue
		}

		if err := provider.Start(ctx); err != nil {
			if a.isStopping() {
				return nil
			}
			setObjectStatus(provider, StatusFailed, err)

			return err
		}
		if a.isStopping() {
			return nil
		}
		setObjectStatus(provider, StatusRunning, nil)
	}

	if serviceCount == 0 {
		return nil
	}
	return a.waitServiceProviders(ctx, serviceCount, serviceResults)
}

func (a *App) waitServiceProviders(ctx context.Context, remaining int, results <-chan providerStartResult) error {
	for remaining > 0 {
		select {
		case result := <-results:
			remaining--
			if a.isStopping() {
				continue
			}
			if ctx != nil && ctx.Err() != nil {
				return errors.Join(ctx.Err(), a.Stop(context.Background()))
			}
			err := result.err
			if err == nil {
				err = fmt.Errorf("%w: %s", ErrProviderServiceExited, providerName(result.provider))
			}
			setObjectStatus(result.provider, StatusFailed, err)
			stopErr := a.Stop(context.Background())
			setObjectStatus(result.provider, StatusFailed, err)
			return errors.Join(err, stopErr)
		case <-ctx.Done():
			if a.isStopping() {
				return nil
			}
			return errors.Join(ctx.Err(), a.Stop(context.Background()))
		}
	}
	return nil
}

func (a *App) stopProviders(ctx context.Context, providers []Provider, keepFailedName string) error {
	var errs []error
	for i := len(providers) - 1; i >= 0; i-- {
		provider := providers[i]
		name := providerName(provider)
		keepFailed := keepFailedName != "" && name == keepFailedName
		if !keepFailed {
			setObjectStatus(provider, StatusStopping, nil)
		}
		if err := stopProvider(ctx, provider); err != nil {
			setObjectStatus(provider, StatusFailed, err)
			errs = append(errs, err)
			continue
		}
		if !keepFailed {
			setObjectStatus(provider, StatusStopped, nil)
		}
	}
	return errors.Join(errs...)
}

func stopProvider(ctx context.Context, provider Provider) error {
	if provider == nil {
		return nil
	}
	return runLifecycle(ctx, func(ctx context.Context) error {
		return provider.Stop(ctx)
	})
}

func (a *App) shutdownCapabilities(ctx context.Context, capabilities []Capability) error {
	var errs []error
	for i := len(capabilities) - 1; i >= 0; i-- {
		capability := capabilities[i]
		if capability == nil {
			continue
		}
		setObjectStatus(capability, StatusStopping, nil)
		if err := runLifecycle(ctx, capability.Shutdown); err != nil {
			setObjectStatus(capability, StatusFailed, err)
			errs = append(errs, err)
			continue
		}
		setObjectStatus(capability, StatusStopped, nil)
	}
	return errors.Join(errs...)
}

func runLifecycle(ctx context.Context, fn func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() {
		done <- fn(ctx)
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func withDefaultStopTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, DefaultStopTimeout)
}

// lockLifecycle serializes construction and destruction without ignoring Stop deadlines.
func (a *App) lockLifecycle(ctx context.Context) (func(), error) {
	a.mu.Lock()
	if a.lifecycleGate == nil {
		a.lifecycleGate = make(chan struct{}, 1)
	}
	gate := a.lifecycleGate
	a.mu.Unlock()
	select {
	case gate <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-gate }) }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
