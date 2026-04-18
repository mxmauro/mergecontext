// See the LICENSE file for license details.

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
	source    []context.Context
	watched   []watchedContext
	lock      sync.RWMutex
	doneCh    chan struct{}
	doneIndex int
	err       error
}

type watchedContext struct {
	index int
	ctx   context.Context
}

// -----------------------------------------------------------------------------

// New creates a new context object from the combination of the provided contexts.
func New(ctx ...context.Context) Context {
	mc := mergeContext{
		lock:      sync.RWMutex{},
		doneIndex: -1,
	}

	if len(ctx) > 0 {
		mc.source = make([]context.Context, len(ctx))
		mc.watched = make([]watchedContext, 0, len(ctx))
		for idx, item := range ctx {
			if item == nil {
				item = context.Background()
			}

			mc.source[idx] = item
			if item.Done() != nil {
				mc.watched = append(mc.watched, watchedContext{
					index: idx,
					ctx:   item,
				})
			}
		}

		if len(mc.watched) > 0 {
			mc.doneCh = make(chan struct{})
			go mc.monitor()
		}
	} else {
		mc.source = []context.Context{context.Background()}
	}

	// Done
	return &mc
}

func (mc *mergeContext) Deadline() (deadline time.Time, ok bool) {
	for _, ctx := range mc.source {
		thisDeadline, thisOk := ctx.Deadline()
		if thisOk {
			if !ok {
				deadline = thisDeadline
				ok = true
			} else if thisDeadline.Before(deadline) {
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
	for _, ctx := range mc.source {
		v := ctx.Value(key)
		if v != nil {
			return v
		}
	}
	return nil
}

func (mc *mergeContext) monitor() {
	once := sync.Once{}
	winner := watchedContext{index: -1}

	gotWinnerCh := make(chan struct{})

	for _, watched := range mc.watched {
		go func(watched watchedContext) {
			select {
			case <-watched.ctx.Done():
				once.Do(func() {
					winner = watched
					close(gotWinnerCh)
				})
			case <-gotWinnerCh:
			}
		}(watched)
	}

	<-gotWinnerCh

	mc.lock.Lock()
	mc.doneIndex = winner.index
	mc.err = winner.ctx.Err()
	mc.lock.Unlock()

	close(mc.doneCh)
}
