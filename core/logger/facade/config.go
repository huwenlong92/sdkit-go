package logger

import (
	corelogger "github.com/huwenlong92/sdkit-go/core/logger"
	"github.com/huwenlong92/sdkit-go/core/runtime"
)

const Name = string(corelogger.KeyLogger)

type RotationConfig = corelogger.RotationConfig
type Config = corelogger.Config

type ConfigLoader func(app *runtime.App) (Config, error)
