package workgroup

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// errBoom is a non-sentinel cause used to exercise wrapping through With.
var errBoom = errors.New("boom")

// TestErrNilSourceAndErrNilWorkerStayMatchableThroughWith names the sentinels'
// claim. They are errs.Const constants so every failure path is matchable with
// errors.Is and never by string comparison — a caller branching on message text
// breaks the moment the wording is improved, and there is nothing to warn them.
// With must preserve BOTH the sentinel and the cause, or wrapping silently
// throws away the identity a caller was going to match on.
func TestErrNilSourceAndErrNilWorkerStayMatchableThroughWith(t *testing.T) {
	tests := []struct {
		err     error
		wantErr error
		name    string
	}{
		{name: "nil source matches itself", err: ErrNilSource, wantErr: ErrNilSource},
		{name: "nil worker matches itself", err: ErrNilWorker, wantErr: ErrNilWorker},
		{name: "With keeps the sentinel matchable", err: ErrNilSource.With(errBoom), wantErr: ErrNilSource},
		{name: "With keeps the cause matchable", err: ErrNilSource.With(errBoom), wantErr: errBoom},
		{
			name:    "With args keeps the sentinel matchable",
			err:     ErrNilWorker.With(nil, "worker", 3),
			wantErr: ErrNilWorker,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ErrorIs(t, tt.err, tt.wantErr)
		})
	}
}

func TestSentinelsAreDistinct(t *testing.T) {
	assert.NotErrorIs(t, ErrNilSource, ErrNilWorker)
	assert.NotErrorIs(t, ErrNilWorker, ErrNilSource)
}
