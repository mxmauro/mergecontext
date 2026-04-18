// See the LICENSE file for license details.

package mergecontext_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"testing"
	"time"

	"github.com/mxmauro/mergecontext"
)

const (
	numberOfContexts = 10
)

type TestContext struct {
	ctxs       [numberOfContexts]context.Context
	cancelCtxs [numberOfContexts]context.CancelFunc

	numList [numberOfContexts]int
}

// -----------------------------------------------------------------------------

func TestMergeCtx_Deadline(t *testing.T) {
	testCtx := NewTestContext()
	defer testCtx.cleanup()

	lowestIndex := 0
	for idx := 0; idx < numberOfContexts; idx++ {
		ctx, cancelCtx := context.WithTimeout(context.Background(), time.Duration(testCtx.num(idx)+2)*time.Second)
		testCtx.assign(idx, ctx, cancelCtx)

		if testCtx.num(idx) == 0 {
			lowestIndex = idx
		}
	}

	ctx := testCtx.mergeCtx()

	<-ctx.Done()

	if lowestIndex != ctx.DoneIndex() {
		t.Fatalf("the soonest context (#%v) is not the one that triggered (#%v)", lowestIndex, ctx.DoneIndex())
	}

	if !errors.Is(testCtx.ctxs[lowestIndex].Err(), context.DeadlineExceeded) {
		t.Fatalf("expected a deadline exceeded error (got: %v)", testCtx.ctxs[lowestIndex].Err().Error())
	}
}

func TestMergeCtx_DeadlineUsesEarliestAbsoluteTime(t *testing.T) {
	base := time.Now().Add(10 * time.Second)

	laterWithLowerNanos, cancelLater := context.WithDeadline(context.Background(), base.Add(2*time.Second))
	defer cancelLater()

	earlierWithHigherNanos, cancelEarlier := context.WithDeadline(context.Background(), base.Add(1500*time.Millisecond))
	defer cancelEarlier()

	ctx := mergecontext.New(laterWithLowerNanos, earlierWithHigherNanos)

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatalf("expected merged context to have a deadline")
	}

	expectedDeadline, _ := earlierWithHigherNanos.Deadline()
	// The earlier absolute deadline must win even if another context has a smaller nanosecond field.
	if !deadline.Equal(expectedDeadline) {
		t.Fatalf("deadline mismatch (found:%v / expected:%v)", deadline, expectedDeadline)
	}
}

func TestMergeCtx_Value(t *testing.T) {
	testCtx := NewTestContext()
	defer testCtx.cleanup()

	for idx := 0; idx < numberOfContexts; idx++ {
		ctx := context.WithValue(context.Background(), getSampleKey(idx), getSampleValue(idx))
		testCtx.assign(idx, ctx, nil)
	}

	ctx := testCtx.mergeCtx()

	for idx := 0; idx < numberOfContexts; idx++ {
		v := ctx.Value(getSampleKey(idx))
		switch _v := v.(type) {
		case string:
			if _v != getSampleValue(idx) {
				t.Fatalf("value mismatch (found:%v / expected:%v)", _v, getSampleValue(idx))
			}

		default:
			t.Fatalf("expected a string value")
		}
	}
}

func TestMergeCtx_NilBecomesBackground(t *testing.T) {
	ctxKey := getSampleKey(0)
	ctxValue := getSampleValue(0)

	valueCtx := context.WithValue(context.Background(), ctxKey, ctxValue)
	ctx := mergecontext.New(nil, valueCtx)

	if ctx == nil {
		t.Fatalf("expected merged context")
	}

	if got := ctx.Value(ctxKey); got != ctxValue {
		t.Fatalf("value mismatch (found:%v / expected:%v)", got, ctxValue)
	}

	select {
	case <-ctx.Done():
		t.Fatalf("merged context should not be done when only the non-nil input is active")
	case <-time.After(25 * time.Millisecond):
	}

	if ctx.DoneIndex() != -1 {
		t.Fatalf("expected done index to remain unset before cancellation (got:%v)", ctx.DoneIndex())
	}

	if ctx.Err() != nil {
		t.Fatalf("expected nil error before cancellation (got:%v)", ctx.Err())
	}
}

func TestMergeCtx_EmptyInputBehavesAsBackground(t *testing.T) {
	ctx := mergecontext.New()

	if ctx == nil {
		t.Fatalf("expected merged context")
	}

	select {
	case <-ctx.Done():
		t.Fatalf("background-like merged context should not be done")
	case <-time.After(25 * time.Millisecond):
	}

	if ctx.DoneIndex() != -1 {
		t.Fatalf("expected done index to remain unset (got:%v)", ctx.DoneIndex())
	}

	if ctx.Err() != nil {
		t.Fatalf("expected nil error (got:%v)", ctx.Err())
	}

	if _, ok := ctx.Deadline(); ok {
		t.Fatalf("background-like merged context should not have a deadline")
	}
}

func TestMergeCtx_CancelledContextWins(t *testing.T) {
	activeCtx, cancelActive := context.WithCancel(context.Background())
	defer cancelActive()

	cancelledCtx, cancelCancelled := context.WithCancel(context.Background())
	cancelCancelled()

	ctx := mergecontext.New(nil, activeCtx, cancelledCtx)

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for merged context cancellation")
	}

	if ctx.DoneIndex() != 2 {
		t.Fatalf("expected cancelled context to win (got:%v)", ctx.DoneIndex())
	}

	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("expected canceled error (got:%v)", ctx.Err())
	}
}

func TestMergeCtx_ReleasesWatcherGoroutines(t *testing.T) {
	before := runtime.NumGoroutine()

	cancelledCtx, cancelCancelled := context.WithCancel(context.Background())
	cancelCancelled()

	ctx := mergecontext.New(
		cancelledCtx,
		context.Background(),
		context.Background(),
		context.Background(),
		context.Background(),
		context.Background(),
		context.Background(),
		context.Background(),
	)

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for merged context cancellation")
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		runtime.Gosched()
		time.Sleep(10 * time.Millisecond)
	}

	after := runtime.NumGoroutine()
	if after > before+2 {
		t.Fatalf("possible goroutine leak detected (before:%v / after:%v)", before, after)
	}
}

// -----------------------------------------------------------------------------

func NewTestContext() *TestContext {
	tc := TestContext{}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for idx := 0; idx < numberOfContexts; idx++ {
		tc.numList[idx] = idx
	}
	r.Shuffle(numberOfContexts, func(i, j int) {
		tc.numList[i], tc.numList[j] = tc.numList[j], tc.numList[i]
	})
	return &tc
}

func (tc *TestContext) assign(idx int, ctx context.Context, cancelCtx context.CancelFunc) {
	tc.ctxs[idx] = ctx
	tc.cancelCtxs[idx] = cancelCtx
}

func (tc *TestContext) cleanup() {
	for idx := 0; idx < numberOfContexts; idx++ {
		if tc.cancelCtxs[idx] != nil {
			tc.cancelCtxs[idx]()
		}
	}
}

func (tc *TestContext) num(idx int) int {
	return tc.numList[idx]
}

func (tc *TestContext) mergeCtx() mergecontext.Context {
	return mergecontext.New(tc.ctxs[:]...)
}

// -----------------------------------------------------------------------------

func getSampleKey(idx int) string {
	return fmt.Sprintf("key%v", idx+1)
}

func getSampleValue(idx int) string {
	return fmt.Sprintf("key%v", idx+1)
}
