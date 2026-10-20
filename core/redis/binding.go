package redis

import (
	"github.com/huwenlong92/sdkit-go/core/runtime"
)

const KeyRedis runtime.Key = "redis"

func From(app *runtime.App) *RuntimeClient {
	if app != nil {
		if value, ok := app.Container().Get(KeyRedis); ok {
			if client, ok := value.(*RuntimeClient); ok {
				return client
			}
		}
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	return Default
}

func Bind(app *runtime.App, client *RuntimeClient) error {
	if app == nil {
		replaceDefault(client)
		return nil
	}
	if client == nil {
		return runtime.ErrContainerValueNil
	}
	return runtime.BindProcessDefault(app, KeyRedis, client, func() { replaceDefault(client) }, func() { replaceDefault(nil) })
}

func replaceDefault(client *RuntimeClient) {
	mu.Lock()
	defer mu.Unlock()

	Default = client
	if client == nil {
		RDB = nil
		return
	}
	RDB = client.Rdb
}
