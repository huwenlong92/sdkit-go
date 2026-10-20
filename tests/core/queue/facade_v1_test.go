package queue_test

import (
	"context"
	"errors"
	"testing"

	"github.com/huwenlong92/sdkit-go/core/queue"
	"github.com/huwenlong92/sdkit-go/core/queue/facade/operations"
	"github.com/huwenlong92/sdkit-go/core/queue/facade/producer"
	"github.com/huwenlong92/sdkit-go/core/runtime"
)

type v1BorrowedProducer struct{ closed int }

func (p *v1BorrowedProducer) Enqueue(context.Context, queue.Task, ...queue.Option) (*queue.TaskInfo, error) {
	return &queue.TaskInfo{ID: "v1"}, nil
}
func (p *v1BorrowedProducer) BatchEnqueue(context.Context, []queue.Task, ...queue.Option) ([]*queue.TaskInfo, error) {
	return nil, nil
}
func (p *v1BorrowedProducer) Close() error { p.closed++; return nil }
func TestV1ProducerResolveAndBorrowing(t *testing.T) {
	client := &v1BorrowedProducer{}
	app := runtime.New()
	if err := app.Use(producer.Use(producer.WithProducer(client))); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(); err != nil {
		t.Fatal(err)
	}
	ctx := runtime.NewServiceContext[any]("worker", "worker", nil)
	ctx.Capabilities.SetBorrowed(producer.Name, producer.From(app))
	resolved, err := producer.Resolve(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolved.Enqueue(context.Background(), queue.Task{Type: "v1"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.closed != 0 {
		t.Fatalf("injected client closed %d times", client.closed)
	}
	if _, err := producer.Resolve(runtime.NewServiceContext[any]("x", "x", nil)); !errors.Is(err, runtime.ErrCapabilityNotFound) {
		t.Fatalf("missing=%v", err)
	}
}
func TestV1OperationsResolve(t *testing.T) {
	ctx := runtime.NewServiceContext[any]("admin", "admin", nil)
	instance := queue.NewRuntimeInstance(&runtimeRunner{})
	ctx.Capabilities.SetBorrowed("admin."+operations.Name, instance)
	resolved, err := operations.Resolve(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolved.ListQueues(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx.Capabilities.Set("admin."+operations.Name, 42)
	if _, err := operations.Resolve(ctx); !errors.Is(err, runtime.ErrCapabilityTypeMismatch) {
		t.Fatalf("type mismatch=%v", err)
	}
}
