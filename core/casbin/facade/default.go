package casbin

import (
	corecasbin "github.com/huwenlong92/sdkit-go/core/casbin"
	"github.com/huwenlong92/sdkit-go/core/runtime"
)

func Default() *Manager {
	return corecasbin.From(nil)
}

func From(app *runtime.App) *Manager {
	return corecasbin.From(app)
}
