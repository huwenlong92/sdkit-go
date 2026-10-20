package payment

import "github.com/huwenlong92/sdkit-go/core/runtime"

const KeyPayment runtime.Key = "payment"

func From(app *runtime.App) *Service {
	if app != nil {
		if value, ok := app.Container().Get(KeyPayment); ok {
			if service, ok := value.(*Service); ok {
				return service
			}
		}
		return nil
	}
	service, _ := Default()
	return service
}

func Bind(app *runtime.App, service *Service) error {
	if app == nil {
		SetDefault(service)
		return nil
	}
	if service == nil {
		return runtime.ErrContainerValueNil
	}
	return runtime.BindProcessDefault(app, KeyPayment, service, func() { SetDefault(service) }, func() { SetDefault(nil) })
}
