//go:build sdkit_queue_nats

package driver

import "github.com/huwenlong92/sdkit-go/pkg/queue/nats"

func init() {
	register("nats", nats.Register)
}
