package eventbus

import (
	coreeventbus "github.com/huwenlong92/sdkit-go/core/eventbus"
	"github.com/huwenlong92/sdkit-go/pkg/eventbus/memory"
)

func init() {
	registerDriverFactory(DriverMemory, func(Config, options) (coreeventbus.Bus, error) {
		return memory.New(), nil
	})
}
