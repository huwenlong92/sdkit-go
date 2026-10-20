package realtime

import "github.com/huwenlong92/sdkit-go/core/runtime"

const KeyRealtime runtime.Key = "realtime"

func From(app *runtime.App) Publisher {
	if app != nil {
		if value, ok := app.Container().Get(KeyRealtime); ok {
			if publisher, ok := value.(Publisher); ok {
				return publisher
			}
		}
		return nil
	}
	return DefaultPublisher()
}

func Bind(app *runtime.App, publisher Publisher) error {
	if app == nil {
		SetDefaultPublisher(publisher)
		return nil
	}
	if publisher == nil {
		return runtime.ErrContainerValueNil
	}
	return runtime.BindProcessDefault(app, KeyRealtime, publisher, func() { SetDefaultPublisher(publisher) }, func() { SetDefaultPublisher(nil) })
}
