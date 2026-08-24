package editor

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

const (
	mascotFrameDelay = 100 * time.Millisecond
	mascotTopRow     = 4
)

type mascotFrame struct {
	lines []string
}

type mascotState struct {
	Position  int
	Direction int
	Frame     int
}

var raccoonFrames = []mascotFrame{
	{lines: []string{
		`  /\_/\\    `,
		` ( o.o )    `,
		` /  ^  \\   `,
		`(  ___  )~~ `,
		` /|   |\\    `,
	}},
	{lines: []string{
		`  /\_/\\    `,
		` ( -.- )    `,
		` /  ^  \\   `,
		`(  ___  )~~ `,
		` /|   |\\    `,
	}},
	{lines: []string{
		`  /\_/\\    `,
		` ( o.o )    `,
		` /  ^  \\   `,
		`(  ___  )~~ `,
		`  /|  |\\    `,
	}},
	{lines: []string{
		`  /\_/\\    `,
		` ( o.o )    `,
		` /  ^  \\   `,
		`(  ___  )~~ `,
		` /|   |\\    `,
	}},
	{lines: []string{
		`  /\_/\\    `,
		` ( ^.^ )    `,
		` /  ^  \\   `,
		`(  ___  )~~ `,
		` /|   |\\    `,
	}},
}

func runMascot(stop <-chan struct{}, done chan<- struct{}, out io.Writer, widthFunc func() int, color bool) {
	defer close(done)
	state := mascotState{Direction: 1}
	ticker := time.NewTicker(mascotFrameDelay)
	defer ticker.Stop()
	render := func() {
		drawMascot(out, state, maxMascotWidth(), widthFunc(), color)
	}
	render()
	for {
		select {
		case <-stop:
			clearMascot(out, maxMascotHeight())
			return
		case <-ticker.C:
			state = nextMascotState(state, widthFunc(), maxMascotWidth(), len(raccoonFrames))
			render()
		}
	}
}

func startMascot(out io.Writer, widthFunc func() int, color bool) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once
	// Render the first frame synchronously. Very fast local fallbacks can finish
	// before a newly started goroutine gets scheduled.
	drawMascot(out, mascotState{Direction: 1}, maxMascotWidth(), widthFunc(), color)
	go runMascot(stop, done, out, widthFunc, color)
	return func() {
		once.Do(func() {
			close(stop)
			<-done
		})
	}
}

func nextMascotState(state mascotState, terminalWidth, artWidth, frameCount int) mascotState {
	maxPosition := maxMascotPosition(terminalWidth, artWidth)
	if state.Direction == 0 {
		state.Direction = 1
	}
	next := state
	next.Position += next.Direction
	if next.Position >= maxPosition {
		next.Position = maxPosition
		next.Direction = -1
	}
	if next.Position <= 0 {
		next.Position = 0
		next.Direction = 1
	}
	if frameCount > 0 {
		next.Frame = (next.Frame + 1) % frameCount
	}
	return next
}

func maxMascotPosition(terminalWidth, artWidth int) int {
	if terminalWidth <= artWidth+1 {
		return 0
	}
	return terminalWidth - artWidth - 1
}

func drawMascot(out io.Writer, state mascotState, artWidth, terminalWidth int, color bool) {
	if len(raccoonFrames) == 0 {
		return
	}
	frame := raccoonFrames[state.Frame%len(raccoonFrames)]
	left := min(state.Position, maxMascotPosition(terminalWidth, artWidth))
	clearMascot(out, maxMascotHeight())
	for i, line := range frame.lines {
		fmt.Fprintf(out, "\033[%d;%dH%s%s\033[0m", mascotTopRow+i, left+1, mascotColor(i, color), line)
	}
}

func clearMascot(out io.Writer, height int) {
	for row := 0; row < height; row++ {
		fmt.Fprintf(out, "\033[%d;1H\033[2K", mascotTopRow+row)
	}
}

func mascotColor(line int, enabled bool) string {
	if !enabled {
		return ""
	}
	switch line % 3 {
	case 0:
		return "\033[33m"
	case 1:
		return "\033[31m"
	default:
		return "\033[32m"
	}
}

func maxMascotWidth() int {
	width := 0
	for _, frame := range raccoonFrames {
		for _, line := range frame.lines {
			width = max(width, len([]rune(line)))
		}
	}
	return width
}

func maxMascotHeight() int {
	height := 0
	for _, frame := range raccoonFrames {
		height = max(height, len(frame.lines))
	}
	return height
}

func hideCursor() { fmt.Print("\033[?25l") }

func showCursor() { fmt.Print("\033[?25h") }

func colorEnabled() bool {
	term := strings.ToLower(strings.TrimSpace(getenv("TERM")))
	return term != "" && term != "dumb"
}
