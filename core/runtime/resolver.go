package runtime

import (
	"errors"
	"fmt"
	"reflect"
)

var ErrCapabilityTypeMismatch = errors.New("runtime: capability type mismatch")
var ErrCapabilityInitialization = errors.New("runtime: capability initialization failed")

// CapabilitySource resolves service-local names before shared names.
// Instances returned by a source are borrowed; resolving never transfers ownership.
type CapabilitySource interface {
	CapabilityLocalFirst(string) (any, bool)
}

// ResolveCapability returns a required instance. Typed nil is treated as a type error.
func ResolveCapability[T any](source CapabilitySource, name string) (T, error) {
	var zero T
	if isNilCapabilityValue(source) {
		return zero, fmt.Errorf("%w: %s", ErrCapabilityNotFound, name)
	}
	value, ok := source.CapabilityLocalFirst(name)
	if !ok {
		return zero, fmt.Errorf("%w: %s", ErrCapabilityNotFound, name)
	}
	typed, ok := value.(T)
	if !ok || isNilCapabilityValue(typed) {
		return zero, fmt.Errorf("%w: %s (got %T)", ErrCapabilityTypeMismatch, name, value)
	}
	return typed, nil
}

// OptionalCapability suppresses absence only; a present but invalid value is an error.
func OptionalCapability[T any](source CapabilitySource, name string) (T, bool, error) {
	value, err := ResolveCapability[T](source, name)
	if errors.Is(err, ErrCapabilityNotFound) {
		return value, false, nil
	}
	return value, err == nil, err
}

// ResolveAppCapability also preserves an initialization failure and its original cause.
func ResolveAppCapability[T any](app *App, name string) (T, error) {
	var zero T
	if app == nil {
		return zero, ErrAppNil
	}
	if health := app.CapabilityStatus(name); health.Status == StatusFailed {
		return zero, fmt.Errorf("%w: %s: %w", ErrCapabilityInitialization, name, health.Error)
	}
	return ResolveCapability[T](app.Container(), name)
}

func (c *Container) CapabilityLocalFirst(name string) (any, bool)               { return c.Get(Key(name)) }
func (r *LocalCapabilityRegistry) CapabilityLocalFirst(name string) (any, bool) { return r.Get(name) }

func isNilCapabilityValue(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
