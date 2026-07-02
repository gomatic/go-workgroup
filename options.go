package workgroup

import "log/slog"

// Optional configures workgroup behavior. apply is a pure transformer: it
// receives the current settings by value and returns the updated value.
type Optional interface {
	apply(settings) settings
}

// onError controls error handling mode (unexported type — only
// FailFast and CollectAll are usable by callers).
type onError int

const (
	FailFast   onError = iota // cancel all on first error (default)
	CollectAll                // continue, join all errors at end
)

func (o onError) apply(s settings) settings {
	s.onError = o
	return s
}

// Workers sets the number of concurrent worker goroutines.
type Workers int

func (w Workers) apply(s settings) settings {
	s.workers = int(w)
	return s
}

// Name identifies the workgroup in log output.
type Name string

func (n Name) apply(s settings) settings {
	s.name = string(n)
	return s
}

// Log sets the structured logger.
type Log struct{ *slog.Logger }

func (l Log) apply(s settings) settings {
	s.logger = l.Logger
	return s
}
