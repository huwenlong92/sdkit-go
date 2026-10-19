package redis

import (
	coreredis "github.com/huwenlong92/sdkit-go/core/redis"
	"github.com/huwenlong92/sdkit-go/core/runtime"
)

const Name = string(coreredis.KeyRedis)

type Config = coreredis.Config
type RuntimeClient = coreredis.RuntimeClient

type ConfigLoader func(app *runtime.App) (Config, error)
