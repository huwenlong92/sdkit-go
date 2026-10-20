package eventbus

import "github.com/huwenlong92/sdkit-go/core/runtime"

const KeyEventBus runtime.Key = "eventbus"

func From(app *runtime.App) Service {
	if app != nil {
		if value, ok := app.Container().Get(KeyEventBus); ok {
			if service, ok := value.(Service); ok {
				return service
			}
		}
		return nil
	}
	return Default()
}

func Bind(app *runtime.App, bus Service) error {
	return BindWithDriver(app, bus, "")
}

func BindWithDriver(app *runtime.App, bus Service, driver string) error {
	if app == nil {
		SetDefaultWithDriver(bus, driver)
		return nil
	}
	if bus == nil {
		return runtime.ErrContainerValueNil
	}
	defaultBus := bus
	if provider, ok := bus.(interface{ Bus() Bus }); ok {
		if inner := provider.Bus(); inner != nil {
			defaultBus = inner
		}
	}
	return runtime.BindProcessDefault(app, KeyEventBus, bus, func() { SetDefaultWithDriver(defaultBus, driver) }, func() { SetDefaultWithDriver(nil, "") })
}
