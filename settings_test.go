package workgroup

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSettings(t *testing.T) {
	logger := slog.Default()
	tests := []struct {
		wantLogger  *slog.Logger
		name        string
		wantName    string
		opts        []Optional
		wantWorkers int
		wantOnError onError
	}{
		{name: "defaults", opts: nil, wantWorkers: runtime.NumCPU(), wantName: "", wantOnError: FailFast, wantLogger: slog.Default()},
		{name: "workers set", opts: []Optional{Workers(4)}, wantWorkers: 4, wantOnError: FailFast, wantLogger: slog.Default()},
		{name: "workers zero clamps to NumCPU", opts: []Optional{Workers(0)}, wantWorkers: runtime.NumCPU(), wantOnError: FailFast, wantLogger: slog.Default()},
		{name: "workers negative clamps to NumCPU", opts: []Optional{Workers(-5)}, wantWorkers: runtime.NumCPU(), wantOnError: FailFast, wantLogger: slog.Default()},
		{name: "name set", opts: []Optional{Name("grp")}, wantWorkers: runtime.NumCPU(), wantName: "grp", wantOnError: FailFast, wantLogger: slog.Default()},
		{name: "collect-all set", opts: []Optional{CollectAll}, wantWorkers: runtime.NumCPU(), wantOnError: CollectAll, wantLogger: slog.Default()},
		{name: "logger set", opts: []Optional{Log{logger}}, wantWorkers: runtime.NumCPU(), wantOnError: FailFast, wantLogger: logger},
		{name: "nil logger clamps to default", opts: []Optional{Log{nil}}, wantWorkers: runtime.NumCPU(), wantOnError: FailFast, wantLogger: slog.Default()},
		{name: "nil option skipped", opts: []Optional{nil, Workers(2), nil}, wantWorkers: 2, wantOnError: FailFast, wantLogger: slog.Default()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSettings(tt.opts)
			assert.Equal(t, tt.wantWorkers, s.workers)
			assert.Equal(t, tt.wantName, s.name)
			assert.Equal(t, tt.wantOnError, s.onError)
			assert.Same(t, tt.wantLogger, s.logger)
		})
	}
}

func TestSettingsLogAttrs(t *testing.T) {
	tests := []struct {
		name      string
		groupName string
		wantLen   int
	}{
		{name: "without name", groupName: "", wantLen: 1},
		{name: "with name", groupName: "grp", wantLen: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := settings{workers: 2, name: tt.groupName}
			attrs := s.logAttrs()
			assert.Len(t, attrs, tt.wantLen)
			assert.Equal(t, "workers", attrs[0].Key)
		})
	}
}

func TestSettingsOutcome(t *testing.T) {
	worker := errors.New("worker failed")
	source := errors.New("source failed")

	tests := []struct {
		sourceErr error
		wantIs    error
		name      string
		errs      []error
		cancel    bool
		wantNil   bool
	}{
		{name: "worker errors take priority", errs: []error{worker}, wantIs: worker},
		{name: "real source error propagates", sourceErr: source, wantIs: source},
		{name: "context source error falls through to ctx", sourceErr: context.Canceled, cancel: true, wantIs: context.Canceled},
		{name: "context cancelled with no errors", cancel: true, wantIs: context.Canceled},
		{name: "clean success", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancel {
				cancel()
			}
			s := settings{workers: 1, logger: slog.Default()}
			err := s.outcome(ctx, s.logAttrs(), tt.errs, tt.sourceErr)
			if tt.wantNil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantIs)
		})
	}
}
