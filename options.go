package workgroup

import "log/slog"

// Optional configures workgroup behavior.
type Optional interface {
	Apply(*settings)
}

// onError controls error handling mode (unexported type — only
// FailFast and CollectAll are usable by callers).
type onError int

const (
	FailFast   onError = iota // cancel all on first error (default)
	CollectAll                // continue, join all errors at end
)

func (o onError) Apply(s *settings) {
	s.onError = o
}

// Workers sets the number of concurrent worker goroutines.
type Workers int

func (w Workers) Apply(s *settings) {
	s.workers = int(w)
}

// Name identifies the workgroup in log output.
type Name string

func (n Name) Apply(s *settings) {
	s.name = string(n)
}

// Log sets the structured logger.
type Log struct{ *slog.Logger }

func (l Log) Apply(s *settings) {
	s.logger = l.Logger
}
