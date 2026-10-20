package storage

import "github.com/huwenlong92/sdkit-go/core/runtime"

const KeyStorage runtime.Key = "storage"

func From(app *runtime.App) *Manager {
	if app != nil {
		if value, ok := app.Container().Get(KeyStorage); ok {
			if manager, ok := value.(*Manager); ok {
				return manager
			}
		}
		return nil
	}
	manager, _ := ManagerDefault()
	return manager
}

func Bind(app *runtime.App, manager *Manager) error {
	if app == nil {
		SetDefault(manager)
		return nil
	}
	if manager == nil {
		return runtime.ErrContainerValueNil
	}
	return runtime.BindProcessDefault(app, KeyStorage, manager, func() { SetDefault(manager) }, func() { SetDefault(nil) })
}
