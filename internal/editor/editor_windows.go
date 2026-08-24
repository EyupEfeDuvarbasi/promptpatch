//go:build windows

package editor

import (
	"os"
	"time"
)

func openTTY() (*os.File, func(), error) {
	return os.Stdin, func() {}, nil
}

// readByteWithin disambiguates a lone Escape key from an arrow-key escape sequence.
func readByteWithin(timeout time.Duration) (byte, bool) {
	result := make(chan byte, 1)
	go func() {
		if value, ok := readByte(); ok {
			result <- value
		}
	}()
	select {
	case value := <-result:
		return value, true
	case <-time.After(timeout):
		return 0, false
	}
}
