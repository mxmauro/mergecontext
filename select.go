package mergectx

import (
	"context"
)

// -----------------------------------------------------------------------------

func multiselect(ctxs []context.Context) int {
	res := make(chan int)

	count := len(ctxs)
	if count == 1 {
		<-ctxs[0].Done()
		return 0
	}

	for offset := 0; offset < count; {
		switch count - offset {
		case 5:
			// This special case is to avoid having a single remaining item at the end of the loop
			go select3(ctxs, res, offset)
			offset += 3
			go select2(ctxs, res, offset)
			offset += 2
		case 3:
			go select3(ctxs, res, offset)
			offset += 3
		case 2:
			go select2(ctxs, res, offset)
			offset += 2
		default:
			go select4(ctxs, res, offset)
			offset += 4
		}
	}

	// Done
	return <-res
}

func select2(ctxs []context.Context, res chan int, offset int) {
	select {
	case <-ctxs[offset].Done():
		res <- offset

	case <-ctxs[offset+1].Done():
		res <- offset + 1
	}
}

func select3(ctxs []context.Context, res chan int, offset int) {
	select {
	case <-ctxs[offset].Done():
		res <- offset

	case <-ctxs[offset+1].Done():
		res <- offset + 1

	case <-ctxs[offset+2].Done():
		res <- offset + 2
	}
}

func select4(ctxs []context.Context, res chan int, offset int) {
	select {
	case <-ctxs[offset].Done():
		res <- offset

	case <-ctxs[offset+1].Done():
		res <- offset + 1

	case <-ctxs[offset+2].Done():
		res <- offset + 2

	case <-ctxs[offset+3].Done():
		res <- offset + 3
	}
}
