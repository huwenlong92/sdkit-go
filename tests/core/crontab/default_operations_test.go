package crontab_test

import (
	"context"
	"errors"
	"testing"

	"github.com/huwenlong92/sdkit-go/core/crontab"
	"github.com/huwenlong92/sdkit-go/core/runtime"
)

type contractOperations struct{ crontab.Operations }

func (contractOperations) ListTemplates(context.Context) ([]crontab.TemplateInfo, error) {
	return []crontab.TemplateInfo{{Key: "contract"}}, nil
}

func TestDefaultOperationsConflictUnbindAndRollback(t *testing.T) {
	for _, tc := range []struct {
		name string
		fail bool
	}{{name: "shutdown"}, {name: "rollback", fail: true}} {
		t.Run(tc.name, func(t *testing.T) {
			app, other := runtime.New(), runtime.New()
			t.Cleanup(func() { _ = app.Stop(context.Background()); _ = other.Stop(context.Background()) })
			if _, err := crontab.ListTemplates(context.Background()); !errors.Is(err, crontab.ErrNotInitialized) {
				t.Fatalf("before run = %v", err)
			}
			cap := runtime.NewCapability("operations", func(app *runtime.App) error { return crontab.BindDefaultOperations(app, contractOperations{}) })
			if err := app.Use(cap); err != nil {
				t.Fatal(err)
			}
			if tc.fail {
				if err := app.Use(runtime.NewCapabilityWithDependencies("failure", []runtime.Dependency{runtime.Require("operations")}, func(*runtime.App) error { return errors.New("contract failure") }, nil)); err != nil {
					t.Fatal(err)
				}
			}
			err := app.Run()
			if tc.fail {
				if err == nil {
					t.Fatal("run succeeded, want failure")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				items, err := crontab.ListTemplates(context.Background())
				if err != nil || len(items) != 1 || items[0].Key != "contract" {
					t.Fatalf("templates = %v, %v", items, err)
				}
				if err := crontab.BindDefaultOperations(other, contractOperations{}); !errors.Is(err, runtime.ErrDefaultConflict) {
					t.Fatalf("conflict = %v", err)
				}
			}
			if err := app.Stop(context.Background()); err != nil {
				t.Fatal(err)
			}
			if _, err := crontab.ListTemplates(context.Background()); !errors.Is(err, crontab.ErrNotInitialized) {
				t.Fatalf("after stop = %v", err)
			}
		})
	}
}
