package tests

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/huwenlong92/sdkit-go/core/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestManagedLoggerCloseDoesNotReopenWriter(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode string
	}{{name: "size", mode: "size"}, {name: "daily", mode: "daily"}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			log, close, err := logger.NewManaged(logger.Config{Name: "contract", RootDir: dir, Rotation: logger.RotationConfig{Mode: tc.mode}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = close() })
			log = log.WithOptions(zap.ErrorOutput(zapcore.AddSync(io.Discard)))
			log.Info("before close")
			matches, err := filepath.Glob(filepath.Join(dir, "contract", "*.log"))
			if err != nil || len(matches) != 1 {
				t.Fatalf("files = %v, %v", matches, err)
			}
			before, err := os.Stat(matches[0])
			if err != nil {
				t.Fatal(err)
			}
			if err := close(); err != nil {
				t.Fatal(err)
			}
			if err := close(); err != nil {
				t.Fatal(err)
			}
			log.Info("after close")
			after, err := os.Stat(matches[0])
			if err != nil {
				t.Fatal(err)
			}
			if before.Size() != after.Size() {
				t.Fatalf("file grew after close: %d -> %d", before.Size(), after.Size())
			}
		})
	}
}
