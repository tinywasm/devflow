package devflow

// Future holds the async result of any initialization.
// It uses any (interface{}) for flexibility without generic syntax.
type Future struct {
	result any
	err    error
	done   chan bool
}

// NewFuture starts async initialization with the given function.
func NewFuture(initFn func() (any, error)) *Future {
	f := &Future{done: make(chan bool)}
	go func() {
		f.result, f.err = initFn()
		f.done <- true
		close(f.done)
	}()
	return f
}

// NewResolvedFuture creates a Future that is already resolved with the given value.
// Useful for tests or when the value is already available synchronously.
func NewResolvedFuture(value any) *Future {
	f := &Future{
		result: value,
		done:   make(chan bool),
	}
	close(f.done) // Already done
	return f
}

// Get blocks until initialization completes and returns the result.
func (f *Future) Get() (any, error) {
	<-f.done
	return f.result, f.err
}

// Ready returns a channel that signals completion.
func (f *Future) Ready() <-chan bool {
	return f.done
}
