package operations

import (
	"github.com/huwenlong92/sdkit-go/core/queue"
	"github.com/huwenlong92/sdkit-go/core/runtime"
)

type ServiceContext interface {
	CapabilityLocalFirst(name string) (any, bool)
}

func From(app *runtime.App) *queue.RuntimeInstance {
	if app == nil {
		return nil
	}
	value, ok := app.Container().Get(runtime.Key(Name))
	if !ok {
		return nil
	}
	runtime, _ := value.(*queue.RuntimeInstance)
	return runtime
}

func RuntimeFrom(app *runtime.App) *queue.RuntimeInstance {
	return From(app)
}

func FromServiceContext(ctx ServiceContext) (*queue.RuntimeInstance, bool) {
	if ctx == nil {
		return nil, false
	}
	value, ok := ctx.CapabilityLocalFirst(Name)
	if !ok {
		return nil, false
	}
	runtime, ok := value.(*queue.RuntimeInstance)
	if !ok || runtime == nil {
		return nil, false
	}
	return runtime, true
}

func RuntimeFromServiceContext(ctx ServiceContext) *queue.RuntimeInstance {
	runtime, _ := FromServiceContext(ctx)
	return runtime
}

func OperationsFromServiceContext(ctx ServiceContext) (*queue.OperationsRuntime, bool) {
	runtime, ok := FromServiceContext(ctx)
	if !ok || runtime.Operations() == nil {
		return nil, false
	}
	return runtime.Operations(), true
}

// Resolve returns only management operations. The capability retains resource ownership.
func Resolve(ctx ServiceContext) (*queue.OperationsRuntime, error) {
	instance, err := runtime.ResolveCapability[*queue.RuntimeInstance](ctx, Name)
	if err != nil {
		return nil, err
	}
	if instance.Operations() == nil {
		return nil, queue.ErrNotInitialized
	}
	return instance.Operations(), nil
}
