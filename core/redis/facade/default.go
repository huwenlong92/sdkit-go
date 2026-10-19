package redis

import (
	coreredis "github.com/huwenlong92/sdkit-go/core/redis"
	"github.com/huwenlong92/sdkit-go/core/runtime"
)

func From(app *runtime.App) *RuntimeClient {
	return coreredis.From(app)
}
