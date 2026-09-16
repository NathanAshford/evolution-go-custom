package registry

import "errors"

var (
	// ErrUnknownCall is returned when a call id has no live manager — it either
	// never existed or already ended.
	ErrUnknownCall = errors.New("call not found or already ended")

	// ErrNoCall is returned when a manager was created but produced no call.
	ErrNoCall = errors.New("call could not be created")
)

// Internal aliases keep the call sites terse.
var (
	errUnknownCall = ErrUnknownCall
	errNoCall      = ErrNoCall
)
