package workgroup

import errs "github.com/gomatic/go-error"

// The package's sentinel errors, declared as constants of go-error's
// [errs.Const] so every path is matchable with errors.Is — never by string
// comparison. Wrapping a cause or appending context goes through
// errs.Const.With(cause, args...), which keeps both the sentinel and the
// cause matchable.
const (
	ErrNilSource errs.Const = "workgroup: nil source"
	ErrNilWorker errs.Const = "workgroup: nil worker"
)
