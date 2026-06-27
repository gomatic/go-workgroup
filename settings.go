package workgroup

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
)

type settings struct {
	logger  *slog.Logger
	name    string
	workers int
	onError onError
}

// must clamps and validates settings after option application.
func (s *settings) must() {
	if s.workers < 1 {
		s.workers = runtime.NumCPU()
	}
	if s.logger == nil {
		s.logger = slog.Default()
	}
}

// logAttrs builds the structured log attributes describing this workgroup.
func (s settings) logAttrs() []slog.Attr {
	attrs := []slog.Attr{slog.Int("workers", s.workers)}
	if s.name != "" {
		attrs = append(attrs, slog.String("name", s.name))
	}
	return attrs
}

// outcome resolves the final error returned by Run from the worker errors,
// the source error, and the context state, logging the terminal condition.
// Worker errors take priority, then a real (non-context) source error, then
// any context cancellation; otherwise the group completed successfully.
func (s settings) outcome(ctx context.Context, attrs []slog.Attr, errs []error, sourceErr error) error {
	if len(errs) > 0 {
		s.logger.LogAttrs(ctx, slog.LevelError, "workgroup completed with errors", attrs...)
		return errors.Join(errs...)
	}
	if sourceErr != nil && !contextError(sourceErr) {
		s.logger.LogAttrs(ctx, slog.LevelError, "workgroup source error", attrs...)
		return sourceErr
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	s.logger.LogAttrs(ctx, slog.LevelInfo, "workgroup completed", attrs...)
	return nil
}

// newSettings creates a settings with defaults, applies options, and validates.
func newSettings(opts []Optional) settings {
	s := settings{
		workers: runtime.NumCPU(),
		logger:  slog.Default(),
		onError: FailFast,
	}
	for _, o := range opts {
		if o != nil {
			o.Apply(&s)
		}
	}
	s.must()
	return s
}
