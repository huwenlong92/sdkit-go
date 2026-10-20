package crontab

import (
	"context"
	"errors"
	"sync"

	"github.com/huwenlong92/sdkit-go/core/runtime"
)

var ErrNotInitialized = errors.New("crontab: operations not initialized")
var defaultOperations struct {
	sync.RWMutex
	operations Operations
}

// BindDefaultOperations borrows operations; the App only owns the default binding.
func BindDefaultOperations(app *runtime.App, operations Operations) error {
	if operations == nil {
		return ErrNotInitialized
	}
	return runtime.BindProcessDefault(app, "crontab.operations.default", operations, func() {
		defaultOperations.Lock()
		defaultOperations.operations = operations
		defaultOperations.Unlock()
	}, func() {
		defaultOperations.Lock()
		defaultOperations.operations = nil
		defaultOperations.Unlock()
	})
}

func Reload(ctx context.Context) error {
	defaultOperations.RLock()
	defer defaultOperations.RUnlock()
	if defaultOperations.operations == nil {
		return ErrNotInitialized
	}
	return defaultOperations.operations.Reload(ctx)
}

func ListTemplates(ctx context.Context) ([]TemplateInfo, error) {
	defaultOperations.RLock()
	defer defaultOperations.RUnlock()
	if defaultOperations.operations == nil {
		return nil, ErrNotInitialized
	}
	return defaultOperations.operations.ListTemplates(ctx)
}

func ListDBTemplates(ctx context.Context) ([]TemplateInfo, error) {
	defaultOperations.RLock()
	defer defaultOperations.RUnlock()
	if defaultOperations.operations == nil {
		return nil, ErrNotInitialized
	}
	return defaultOperations.operations.ListDBTemplates(ctx)
}

func ListEntries(ctx context.Context) ([]EntryInfo, error) {
	defaultOperations.RLock()
	defer defaultOperations.RUnlock()
	if defaultOperations.operations == nil {
		return nil, ErrNotInitialized
	}
	return defaultOperations.operations.ListEntries(ctx)
}

func GetEntry(ctx context.Context, id string) (EntryInfo, error) {
	defaultOperations.RLock()
	defer defaultOperations.RUnlock()
	if defaultOperations.operations == nil {
		return EntryInfo{}, ErrNotInitialized
	}
	return defaultOperations.operations.GetEntry(ctx, id)
}

func CreateEntry(ctx context.Context, req CreateEntryRequest) (EntryInfo, error) {
	defaultOperations.RLock()
	defer defaultOperations.RUnlock()
	if defaultOperations.operations == nil {
		return EntryInfo{}, ErrNotInitialized
	}
	return defaultOperations.operations.CreateEntry(ctx, req)
}

func UpdateEntry(ctx context.Context, id string, req UpdateEntryRequest) error {
	defaultOperations.RLock()
	defer defaultOperations.RUnlock()
	if defaultOperations.operations == nil {
		return ErrNotInitialized
	}
	return defaultOperations.operations.UpdateEntry(ctx, id, req)
}

func DeleteEntry(ctx context.Context, id string) error {
	defaultOperations.RLock()
	defer defaultOperations.RUnlock()
	if defaultOperations.operations == nil {
		return ErrNotInitialized
	}
	return defaultOperations.operations.DeleteEntry(ctx, id)
}

func EnableEntry(ctx context.Context, id string) error {
	defaultOperations.RLock()
	defer defaultOperations.RUnlock()
	if defaultOperations.operations == nil {
		return ErrNotInitialized
	}
	return defaultOperations.operations.EnableEntry(ctx, id)
}

func DisableEntry(ctx context.Context, id string) error {
	defaultOperations.RLock()
	defer defaultOperations.RUnlock()
	if defaultOperations.operations == nil {
		return ErrNotInitialized
	}
	return defaultOperations.operations.DisableEntry(ctx, id)
}

func RunOnce(ctx context.Context, req RunOnceRequest) error {
	defaultOperations.RLock()
	defer defaultOperations.RUnlock()
	if defaultOperations.operations == nil {
		return ErrNotInitialized
	}
	return defaultOperations.operations.RunOnce(ctx, req)
}

func ListRuntime(ctx context.Context) ([]RuntimeInfo, error) {
	defaultOperations.RLock()
	defer defaultOperations.RUnlock()
	if defaultOperations.operations == nil {
		return nil, ErrNotInitialized
	}
	return defaultOperations.operations.ListRuntime(ctx)
}

func GetRuntime(ctx context.Context, id string) (RuntimeInfo, error) {
	defaultOperations.RLock()
	defer defaultOperations.RUnlock()
	if defaultOperations.operations == nil {
		return RuntimeInfo{}, ErrNotInitialized
	}
	return defaultOperations.operations.GetRuntime(ctx, id)
}

func ListRuns(ctx context.Context, filter RunFilter) ([]RunLog, error) {
	defaultOperations.RLock()
	defer defaultOperations.RUnlock()
	if defaultOperations.operations == nil {
		return nil, ErrNotInitialized
	}
	return defaultOperations.operations.ListRuns(ctx, filter)
}

func ListRunLogs(ctx context.Context, filter RunLogFilter) ([]LogEvent, error) {
	defaultOperations.RLock()
	defer defaultOperations.RUnlock()
	if defaultOperations.operations == nil {
		return nil, ErrNotInitialized
	}
	return defaultOperations.operations.ListRunLogs(ctx, filter)
}

func StreamLogs(ctx context.Context, filter LogFilter) (<-chan LogEvent, error) {
	defaultOperations.RLock()
	defer defaultOperations.RUnlock()
	if defaultOperations.operations == nil {
		return nil, ErrNotInitialized
	}
	return defaultOperations.operations.StreamLogs(ctx, filter)
}
