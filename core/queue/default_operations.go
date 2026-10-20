package queue

import (
	"context"
	"sync"

	"github.com/huwenlong92/sdkit-go/core/runtime"
)

var defaultOperations struct {
	sync.RWMutex
	instance *RuntimeInstance
}

// BindDefaultOperations publishes a borrowed operations instance owned by app.
// Shutdown waits for in-flight package calls before the capability closes resources.
func BindDefaultOperations(app *runtime.App, instance *RuntimeInstance) error {
	if instance == nil || instance.Operations() == nil {
		return ErrNotInitialized
	}
	return runtime.BindProcessDefault(app, "queue.operations.default", instance, func() {
		defaultOperations.Lock()
		defaultOperations.instance = instance
		defaultOperations.Unlock()
	}, func() {
		defaultOperations.Lock()
		defaultOperations.instance = nil
		defaultOperations.Unlock()
	})
}

func acquireRuntime(ctx context.Context) (*RuntimeInstance, func()) {
	if instance := Runtime(ctx); instance != nil {
		return instance, func() {}
	}
	defaultOperations.RLock()
	return defaultOperations.instance, defaultOperations.RUnlock
}

func Status(ctx context.Context) ([]*QueueInfo, error) {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return nil, ErrNotInitialized
	}
	return instance.Operations().Status(ctx)
}

func RuntimeStatus(ctx context.Context) (*RuntimeStatusInfo, error) {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return nil, ErrNotInitialized
	}
	return instance.Operations().RuntimeStatus(ctx)
}

func QueueStatus(ctx context.Context) ([]QueueRuntimeStatus, error) {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return nil, ErrNotInitialized
	}
	return instance.Operations().QueueStatus(ctx)
}

func Metrics(ctx context.Context) (*RuntimeMetrics, error) {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return nil, ErrNotInitialized
	}
	return instance.Operations().Metrics(ctx)
}

func ListQueues(ctx context.Context) ([]*QueueInfo, error) {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return nil, ErrNotInitialized
	}
	return instance.Operations().ListQueues(ctx)
}

func GetQueue(ctx context.Context, queueName string) (*QueueInfo, error) {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return nil, ErrNotInitialized
	}
	return instance.Operations().GetQueue(ctx, queueName)
}

func ListTasks(ctx context.Context, query TaskQuery) ([]*TaskInfo, error) {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return nil, ErrNotInitialized
	}
	return instance.Operations().ListTasks(ctx, query)
}

func GetTask(ctx context.Context, queueName string, taskID string) (*TaskInfo, error) {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return nil, ErrNotInitialized
	}
	return instance.Operations().GetTask(ctx, queueName, taskID)
}

func DeleteTask(ctx context.Context, queueName string, taskID string) error {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return ErrNotInitialized
	}
	return instance.Operations().DeleteTask(ctx, queueName, taskID)
}

func RetryTask(ctx context.Context, queueName string, taskID string) error {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return ErrNotInitialized
	}
	return instance.Operations().RetryTask(ctx, queueName, taskID)
}

func ArchiveTask(ctx context.Context, queueName string, taskID string) error {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return ErrNotInitialized
	}
	return instance.Operations().ArchiveTask(ctx, queueName, taskID)
}

func CancelTask(ctx context.Context, queueName string, taskID string) error {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return ErrNotInitialized
	}
	return instance.Operations().CancelTask(ctx, queueName, taskID)
}

func PauseQueue(ctx context.Context, queueName string) error {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return ErrNotInitialized
	}
	return instance.Operations().PauseQueue(ctx, queueName)
}

func ResumeQueue(ctx context.Context, queueName string) error {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return ErrNotInitialized
	}
	return instance.Operations().ResumeQueue(ctx, queueName)
}

func DrainQueue(ctx context.Context, queueName string) error {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return ErrNotInitialized
	}
	return instance.Operations().DrainQueue(ctx, queueName)
}

func Drain(ctx context.Context) error {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return ErrNotInitialized
	}
	return instance.Operations().Drain(ctx)
}

func ListFailedTasks(ctx context.Context, query TaskQuery) ([]*TaskInfo, error) {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return nil, ErrNotInitialized
	}
	return instance.Operations().ListFailedTasks(ctx, query)
}

func CleanTasks(ctx context.Context, query TaskQuery) (int, error) {
	instance, release := acquireRuntime(ctx)
	defer release()
	if instance == nil {
		return 0, ErrNotInitialized
	}
	return instance.Operations().CleanTasks(ctx, query)
}
