package mergecontext

import (
	"context"
	"sync"
	"time"
)

// -----------------------------------------------------------------------------

type Context interface {
	context.Context

	DoneIndex() int
}

// -----------------------------------------------------------------------------

type mergeContext struct {
	ctxs      []context.Context
	lock      sync.RWMutex
	doneCh    chan struct{}
	doneIndex int
	err       error
}

// -----------------------------------------------------------------------------

// New creates a new context object from the combination of the provided contexts.
func New(ctx ...context.Context) Context {
	if len(ctx) == 0 {
		return nil
	}

	// Create new context merger
	mc := mergeContext{
		ctxs:      ctx,
		lock:      sync.RWMutex{},
		doneCh:    make(chan struct{}),
		doneIndex: -1,
	}
	go mc.monitor()

	// Done
	return &mc
}

func (mc *mergeContext) Deadline() (deadline time.Time, ok bool) {
	for _, ctx := range mc.ctxs {
		thisDeadline, thisOk := ctx.Deadline()
		if thisOk {
			if !ok {
				deadline = thisDeadline
				ok = true
			} else if thisDeadline.Nanosecond() < deadline.Nanosecond() {
				deadline = thisDeadline
			}
		}
	}
	return
}

func (mc *mergeContext) Done() <-chan struct{} {
	return mc.doneCh
}

func (mc *mergeContext) DoneIndex() int {
	mc.lock.RLock()
	defer mc.lock.RUnlock()

	return mc.doneIndex
}

func (mc *mergeContext) Err() error {
	mc.lock.RLock()
	defer mc.lock.RUnlock()

	return mc.err
}

func (mc *mergeContext) Value(key any) any {
	for _, ctx := range mc.ctxs {
		v := ctx.Value(key)
		if v != nil {
			return v
		}
	}
	return nil
}

func (mc *mergeContext) monitor() {
	winner := multiselect(mc.ctxs)

	mc.lock.Lock()
	mc.doneIndex = winner
	mc.err = mc.ctxs[winner].Err()
	mc.lock.Unlock()

	close(mc.doneCh)
}
