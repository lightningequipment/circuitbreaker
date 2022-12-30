package main

import (
	"os"
	"runtime/pprof"
	"time"
)

const testTimeout = 5 * time.Second

// Timeout implements a test level timeout.
func Timeout() func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-time.After(testTimeout):
			_ = pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)

			panic("test timeout")
		case <-done:
		}
	}()

	return func() {
		close(done)
	}
}
