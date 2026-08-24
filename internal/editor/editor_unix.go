//go:build !windows

package editor

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func openTTY() (*os.File, func(), error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	return tty, func() { _ = tty.Close() }, nil
}

// readByteWithin disambiguates a lone Escape key from an arrow-key escape sequence.
func readByteWithin(timeout time.Duration) (byte, bool) {
	ready, err := unix.Poll([]unix.PollFd{{Fd: int32(terminalInput.Fd()), Events: unix.POLLIN}}, int(timeout.Milliseconds()))
	if err != nil || ready == 0 {
		return 0, false
	}
	return readByte()
}
