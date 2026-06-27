package workgroup

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errBoom is a non-sentinel cause used to exercise With's wrapping branch.
var errBoom = errors.New("boom")

func TestError(t *testing.T) {
	assert.Equal(t, "workgroup: nil source", ErrNilSource.Error())
}

func TestErrorWith(t *testing.T) {
	tests := []struct {
		cause    error
		name     string
		sentinel Error
		wantMsg  string
		args     []any
	}{
		{
			name:     "no args, nil cause returns the sentinel itself",
			sentinel: ErrNilSource,
			cause:    nil,
			args:     nil,
			wantMsg:  "workgroup: nil source",
		},
		{
			name:     "no args, with cause wraps both sentinel and cause",
			sentinel: ErrNilWorker,
			cause:    errBoom,
			args:     nil,
			wantMsg:  "workgroup: nil worker: boom",
		},
		{
			name:     "with args, nil cause appends context and wraps the sentinel",
			sentinel: ErrNilSource,
			cause:    nil,
			args:     []any{"detail"},
			wantMsg:  "workgroup: nil source: detail",
		},
		{
			name:     "with cause and args, cause precedes space-separated args",
			sentinel: ErrNilWorker,
			cause:    errBoom,
			args:     []any{"file", "in.txt"},
			wantMsg:  "workgroup: nil worker: boom: file in.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sentinel.With(tt.cause, tt.args...)
			require.Error(t, err)
			assert.Equal(t, tt.wantMsg, err.Error())
			// The sentinel is ALWAYS matchable through With.
			assert.ErrorIs(t, err, tt.sentinel)
			// The cause is matchable whenever one was provided.
			if tt.cause != nil {
				assert.ErrorIs(t, err, tt.cause)
			}
		})
	}
}
