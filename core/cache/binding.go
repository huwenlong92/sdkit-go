package cache

import (
	"github.com/huwenlong92/sdkit-go/core/runtime"
)

const KeyCache runtime.Key = "cache"

func From(app *runtime.App) Cache {
	if app != nil {
		if value, ok := app.Container().Get(KeyCache); ok {
			if c, ok := value.(Cache); ok {
				return c
			}
		}
		return nil
	}
	return Default()
}

func Bind(app *runtime.App, c Cache) error {
	if app == nil {
		replaceDefault(c)
		return nil
	}
	if c == nil {
		return runtime.ErrContainerValueNil
	}
	return runtime.BindProcessDefault(app, KeyCache, c, func() { replaceDefault(c) }, func() { replaceDefault(nil) })
}
