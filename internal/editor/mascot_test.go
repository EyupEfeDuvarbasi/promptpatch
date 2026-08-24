package editor

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestMascotTurnsAroundAtRightBoundary(t *testing.T) {
	state := mascotState{Position: 8, Direction: 1}
	next := nextMascotState(state, 20, 10, 4)
	if next.Position != 9 || next.Direction != -1 {
		t.Fatalf("state=%#v", next)
	}
}

func TestMascotTurnsAroundAtLeftBoundary(t *testing.T) {
	state := mascotState{Position: 0, Direction: -1}
	next := nextMascotState(state, 20, 10, 4)
	if next.Position != 0 || next.Direction != 1 {
		t.Fatalf("state=%#v", next)
	}
}

func TestMaxMascotPositionUsesTerminalWidth(t *testing.T) {
	if got := maxMascotPosition(80, 12); got != 67 {
		t.Fatalf("position=%d", got)
	}
	if got := maxMascotPosition(10, 12); got != 0 {
		t.Fatalf("narrow position=%d", got)
	}
}

func TestMascotStopsOnSignal(t *testing.T) {
	stop := make(chan struct{})
	done := make(chan struct{})
	var output bytes.Buffer
	go runMascot(stop, done, &output, func() int { return 80 }, false)
	close(stop)
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("mascot did not stop")
	}
	if !strings.Contains(output.String(), "\033[2K") {
		t.Fatalf("expected cleanup output, got %q", output.String())
	}
}

func TestMascotColorCanBeDisabled(t *testing.T) {
	var output bytes.Buffer
	drawMascot(&output, mascotState{Direction: 1}, maxMascotWidth(), 80, false)
	if strings.Contains(output.String(), "\033[33m") || strings.Contains(output.String(), "\033[31m") || strings.Contains(output.String(), "\033[32m") {
		t.Fatalf("color leaked when disabled: %q", output.String())
	}
}

func TestMascotRendersRaccoon(t *testing.T) {
	var output bytes.Buffer
	drawMascot(&output, mascotState{Direction: 1}, maxMascotWidth(), 80, false)
	if !strings.Contains(output.String(), "/\\_/\\\\") || !strings.Contains(output.String(), "( o.o )") {
		t.Fatalf("raccoon was not rendered: %q", output.String())
	}
}

func TestStartMascotRendersBeforeFastStop(t *testing.T) {
	var output bytes.Buffer
	stop := startMascot(&output, func() int { return 80 }, false)
	stop()
	if !strings.Contains(output.String(), "( o.o )") {
		t.Fatalf("raccoon was not rendered before stop: %q", output.String())
	}
}
