package logger

import (
	"io"
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewManaged creates an independent logger and returns its resource owner.
// Closing waits for current writes and prevents later writes from reopening files.
// Injected loggers remain owned by their caller.
func NewManaged(cfg Config) (*zap.Logger, func() error, error) {
	cfg = normalizeConfig(cfg)
	writer, err := writerWithConfig(cfg, cfg.Name, cfg.Name+".log")
	if err != nil {
		return nil, nil, err
	}
	owned := &managedWriter{writer: writer}
	console := os.Stderr
	if cfg.Mode == "dev" {
		console = os.Stdout
	}
	sink := zapcore.NewMultiWriteSyncer(zapcore.AddSync(console), zapcore.AddSync(owned))
	core := zapcore.NewCore(newEncoder(cfg), sink, zap.NewAtomicLevelAt(parseZapLevel(cfg.Level)))
	return zap.New(core, zap.AddCaller()), owned.Close, nil
}

type managedWriter struct {
	sync.Mutex
	writer io.Writer
	closed bool
}

func (w *managedWriter) Write(data []byte) (int, error) {
	w.Lock()
	defer w.Unlock()
	if w.closed {
		return 0, os.ErrClosed
	}
	return w.writer.Write(data)
}
func (w *managedWriter) Close() error {
	w.Lock()
	defer w.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	if closer, ok := w.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
