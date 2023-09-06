package mergectx_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
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

func (tc *TestContext) mergeCtx() mergectx.Context {
	return mergectx.New(tc.ctxs[:]...)
}

// -----------------------------------------------------------------------------

func getSampleKey(idx int) string {
	return fmt.Sprintf("key%v", idx+1)
}

func getSampleValue(idx int) string {
	return fmt.Sprintf("key%v", idx+1)
}
