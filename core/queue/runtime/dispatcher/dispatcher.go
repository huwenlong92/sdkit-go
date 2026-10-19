package dispatcher

import "github.com/huwenlong92/sdkit-go/core/queue"

type Dispatcher = queue.Dispatcher
type HandlerMetadata = queue.HandlerMetadata

func New() *Dispatcher {
	return queue.NewDispatcher()
}
