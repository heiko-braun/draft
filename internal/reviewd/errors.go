package reviewd

import "errors"

// ErrNotFound indicates the requested entity does not exist.
var ErrNotFound = errors.New("not found")

// ErrVersionConflict indicates an optimistic concurrency conflict.
var ErrVersionConflict = errors.New("version conflict")
