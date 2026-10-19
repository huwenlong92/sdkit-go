package tests

import (
	"github.com/huwenlong92/sdkit-go/core/eventbus"
	"github.com/huwenlong92/sdkit-go/pkg/eventbus/memory"
)

var (
	_ eventbus.Service = memory.New()
	_ eventbus.Bus     = memory.New()
)
