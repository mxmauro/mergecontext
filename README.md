# mergecontext

Combines a set of Golang `context.Context` objects into one.

### Behavior

The returned context will act according to the following rules:

1. It is fulfilled as soon as one of the components is fulfilled.
2. If it involves an error, the merged object will return the same error.
3. The context object has an additional method that returns the component index that was signalled.


## Example

```golang
import (
    "github.com/mxmauro/mergecontext"
)

func main() {
    ctx1, cancelCtx1 := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancelCtx1()
    ctx2, cancelCtx2 := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancelCtx2()
    ctx3, cancelCtx3 := context.WithTimeout(context.Background(), 8*time.Second)
    defer cancelCtx3()

    ctx := mergectx.New(ctx1, ctx2, ctx3)

    <-ctx.Done()

    if ctx.DoneIndex() != 1 {
        // Error: The second context should be signalled
    }

    // ....
}
```

## LICENSE

See the [license](LICENSE) file for details.
