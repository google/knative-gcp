package ingress

import "errors"

// ErrNotFound is the error when a broker doesn't exist in the configmap.
// This can happen if the clients specifies invalid broker in the path, or the configmap volume hasn't been updated.
var ErrNotFound = errors.New("not found")

// ErrIncomplete is the error when a broker entry exists in the configmap but its decouple queue is nil or empty.
// This should never happen unless there is a bug in the controller.
var ErrIncomplete = errors.New("incomplete config")
