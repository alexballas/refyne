package common

import (
	"github.com/alexballas/refyne/v2/internal/async"
)

var DonePool = async.Pool[chan struct{}]{
	New: func() chan struct{} {
		return make(chan struct{})
	},
}
