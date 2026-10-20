package crontab

import "github.com/huwenlong92/sdkit-go/core/runtime"

const OperationsCapabilityName = "crontab.operations"

// ResolveOperations resolves a borrowed management API from a service context.
// Pass an explicit name for a custom capability registration.
func ResolveOperations(source runtime.CapabilitySource, names ...string) (Operations, error) {
	name := OperationsCapabilityName
	if len(names) > 0 && names[0] != "" {
		name = names[0]
	}
	return runtime.ResolveCapability[Operations](source, name)
}
