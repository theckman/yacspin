// Package yacspin provides Yet Another CLi Spinner for Go, taking inspiration
// (and some utility code) from the https://github.com/briandowns/spinner
// project. Specifically this project borrows the default character sets, and
// color mappings to github.com/fatih/color colors, from that project.
//
// This spinner should support all major operating systems, and is tested
// against Linux, MacOS, and Windows.
//
// This spinner also supports an alternate mode of operation when the TERM
// environment variable is set to "dumb". This is discovered automatically when
// constructing the spinner.
//
// Within the yacspin package there are some default spinners stored in the
// yacspin.CharSets variable, and you can also provide your own. There is also a
// list of known colors in the yacspin.ValidColors variable, if you'd like to
// see what's supported. If you've used github.com/fatih/color before, they
// should look familiar.
//
//		cfg := yacspin.Config{
//			Frequency:     100 * time.Millisecond,
//			CharSet:       yacspin.CharSets[59],
//			Suffix:        " backing up database to S3",
//			Message:       "exporting data",
//			StopCharacter: "✓",
//			StopColors:    []string{"fgGreen"},
//		}
//
//		spinner, err := yacspin.New(cfg)
//		// handle the error
//
//		spinner.Start()
//
//		// doing some work
//		time.Sleep(2 * time.Second)
//
//		spinner.Message("uploading data")
//
//		// upload...
//		time.Sleep(2 * time.Second)
//
//		spinner.Stop()
//
// Check out the Config struct to see all of the possible configuration options
// supported by the Spinner.
package yacspin

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/mattn/go-runewidth"
)

type character struct {
	Value string
	Size  int
}

func setToCharSlice(ss []string) ([]character, int) {
	if len(ss) == 0 {
		return nil, 0
	}

	var maxWidth int
	c := make([]character, len(ss))

	for i, s := range ss {
		n := runewidth.StringWidth(s)
		if n > maxWidth {
			maxWidth = n
		}

		c[i] = character{
			Value: s,
			Size:  n,
		}
	}

	return c, maxWidth
}

// Config is the configuration structure for the Spinner.
type Config struct {
	// Frequency specifies how often to animate the spinner. Optimal value
	// depends on the character set you use.
	//
	// Note: This is a required value (cannot be 0).
	Frequency time.Duration

	// Writer is the place where we are outputting the spinner, and can't be
	// changed after the *Spinner has been constructed. If omitted (nil), this
	// defaults to os.Stdout.
	Writer io.Writer

	// ShowCursor specifies that the cursor should be shown by the spinner while
	// animating. If it is not shown, the cursor will be restored when the
	// spinner stops. This can't be changed after the *Spinner has been
	// constructed.
	//
	// Please note, if you don't show the cursor and the program crashes or is
	// killed, you may need to reset your terminal for the cursor to appear
	// again.
	ShowCursor bool

	// HideCursor describes whether the cursor should be hidden by the spinner
	// while animating. If it is hidden, it will be restored when the spinner
	// stops. This can't be changed after the *Spinner has been constructed.
	//
	// Please note, if the program crashes or is killed you may need to reset
	// your terminal for the cursor to appear again.
	//
	// Deprecated: use ShowCursor instead.
	HideCursor bool

	// SpinnerAtEnd configures the spinner to render the animation at the end of
	// the line instead of the beginning. The default behavior is to render the
	// animated spinner at the beginning of the line.
	SpinnerAtEnd bool

	// ColorAll describes whether to color everything (all) or just the spinner
	// character(s). This cannot be changed after the *Spinner has been
	// constructed.
	ColorAll bool

	// Colors are the colors used for the different printed messages. This
	// respects the ColorAll field.
	Colors []string

	// CharSet is the list of characters to iterate through to draw the spinner.
	CharSet []string

	// Prefix is the string printed immediately before the spinner.
	//
	// If SpinnerAtEnd is set to true, it's recommended that this string start
	// with a space character (` `).
	Prefix string

	// Suffix is the string printed immediately after the spinner and before the
	// message.
	//
	// If SpinnerAtEnd is set to false, it's recommended that this string starts
	// with an space character (` `).
	Suffix string

	// SuffixAutoColon configures whether the spinner adds a colon after the
	// suffix automatically. If there is a message, a colon followed by a space
	// is added to the suffix. Otherwise, if there is no message the colon is
	// omitted.
	//
	// If SpinnerAtEnd is set to true, this option is ignored.
	SuffixAutoColon bool

	// Message is the message string printed by the spinner. If SpinnerAtEnd is
	// set to false and SuffixAutoColon is set to true, the printed line will
	// look like:
	//
	//    <prefix><spinner><suffix>: <message>
	//
	// If SpinnerAtEnd is set to true, the printed line will instead look like
	// this:
	//
	//    <message><prefix><spinner><suffix>
	//
	// In this case, it may be preferred to set the Prefix to empty space (` `).
	Message string

	// StopMessage is the message used when Stop() is called.
	StopMessage string

	// StopCharacter is spinner character used when Stop() is called.
	// Recommended character is ✓, and can be more than just one character.
	StopCharacter string

	// StopColors are the colors used for the Stop() printed line. This respects
	// the ColorAll field.
	StopColors []string

	// StopFailMessage is the message used when StopFail() is called.
	StopFailMessage string

	// StopFailCharacter is the spinner character used when StopFail() is
	// called. Recommended character is ✗, and can be more than just one
	// character.
	StopFailCharacter string

	// StopFailColors are the colors used for the StopFail() printed line. This
	// respects the ColorAll field.
	StopFailColors []string

	// NotTTY tells the spinner that the Writer should not be treated as a TTY.
	// This results in the animation being disabled, with the animation only
	// happening whenever the data is updated. This mode also renders each
	// update on new line, versus reusing the current line.
	NotTTY bool
}

// Spinner is a type representing an animated CLi terminal spinner. It's
// configured via the Config struct type, and controlled via its methods. Some
// of its configuration can also be updated via methods.
//
// Note: You need to use New() to construct a *Spinner.
type Spinner struct {
	writer          io.Writer
	buffer          *bytes.Buffer
	colorAll        bool
	cursorHidden    bool
	suffixAutoColon bool
	isDumbTerm      bool
	isNotTTY        bool
	spinnerAtEnd    bool

	status       *uint32
	lastPrintLen int
	cancelCh     chan struct{} // send: Stop(), close: StopFail(); both stop painter
	doneCh       chan struct{}
	pauseCh      chan struct{}
	unpauseCh    chan struct{}
	unpausedCh   chan struct{}

	// mutex hat and the fields wearing it
	mu                *sync.Mutex
	frequency         time.Duration
	chars             []character
	maxWidth          int
	index             int
	prefix            string
	suffix            string
	message           string
	colorFn           func(format string, a ...interface{}) string
	stopMsg           string
	stopChar          character
	stopColorFn       func(format string, a ...interface{}) string
	stopFailMsg       string
	stopFailChar      character
	stopFailColorFn   func(format string, a ...interface{}) string
	frequencyUpdateCh chan time.Duration
	dataUpdateCh      chan struct{}
}

const (
	statusStopped uint32 = iota
	statusStarting
	statusRunning
	statusStopping
	statusPausing
	statusPaused
	statusUnpausing
)

// New creates a new unstarted spinner. If stdout does not appear to be a TTY,
// this constructor implicitly sets cfg.NotTTY to true.
func New(cfg Config) (*Spinner, error) {
	if cfg.Frequency < 1 {
		return nil, errors.New("cfg.Frequency must be greater than 0")
	}

	if cfg.ShowCursor && cfg.HideCursor {
		return nil, errors.New("cfg.ShowCursor and cfg.HideCursor cannot be true")
	}

	if cfg.HideCursor {
		cfg.ShowCursor = false
	}

	if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		cfg.NotTTY = true
	}

	buf := bytes.NewBuffer(make([]byte, 2048))
	buf.Reset()

	s := &Spinner{
		buffer:            buf,
		mu:                &sync.Mutex{},
		frequency:         cfg.Frequency,
		status:            uint32Ptr(0),
		frequencyUpdateCh: make(chan time.Duration), // use unbuffered for now to avoid .Frequency() panic
		dataUpdateCh:      make(chan struct{}),

		colorAll:        cfg.ColorAll,
		cursorHidden:    !cfg.ShowCursor,
		spinnerAtEnd:    cfg.SpinnerAtEnd,
		suffixAutoColon: cfg.SuffixAutoColon,
		isDumbTerm:      os.Getenv("TERM") == "dumb",
		colorFn:         fmt.Sprintf,
		stopColorFn:     fmt.Sprintf,
		stopFailColorFn: fmt.Sprintf,
	}

	if err := s.Colors(cfg.Colors...); err != nil {
		return nil, err
	}

	if err := s.StopColors(cfg.StopColors...); err != nil {
		return nil, err
	}

	if err := s.StopFailColors(cfg.StopFailColors...); err != nil {
		return nil, err
	}

	if len(cfg.CharSet) == 0 {
		cfg.CharSet = CharSets[9]
	}

	// can only error if the charset is empty, and we prevent that above
	_ = s.CharSet(cfg.CharSet)

	if cfg.NotTTY {
		s.isNotTTY = true
		s.isDumbTerm = true
	}

	if cfg.Writer == nil {
		cfg.Writer = colorable.NewColorableStdout()
	}

	s.writer = cfg.Writer

	if len(cfg.Prefix) > 0 {
		s.Prefix(cfg.Prefix)
	}

	if len(cfg.Suffix) > 0 {
		s.Suffix(cfg.Suffix)
	}

	if len(cfg.Message) > 0 {
		s.Message(cfg.Message)
	}

	if len(cfg.StopMessage) > 0 {
		s.StopMessage(cfg.StopMessage)
	}

	if len(cfg.StopCharacter) > 0 {
		s.StopCharacter(cfg.StopCharacter)
	}

	if len(cfg.StopFailMessage) > 0 {
		s.StopFailMessage(cfg.StopFailMessage)
	}

	if len(cfg.StopFailCharacter) > 0 {
		s.StopFailCharacter(cfg.StopFailCharacter)
	}

	return s, nil
}

func (s *Spinner) notifyDataChange() {
	// non-blocking notification
	select {
	case s.dataUpdateCh <- struct{}{}:
	default:
	}
}

// SpinnerStatus describes the status of the spinner. See the package constants
// for the list of all possible statuses
type SpinnerStatus uint32

const (
	// SpinnerStopped is a stopped spinner
	SpinnerStopped SpinnerStatus = iota

	// SpinnerStarting is a starting spinner
	SpinnerStarting

	// SpinnerRunning is a running spinner
	SpinnerRunning

	// SpinnerStopping is a stopping spinner
	SpinnerStopping

	// SpinnerPausing is a pausing spinner
	SpinnerPausing

	// SpinnerPaused is a paused spinner
	SpinnerPaused

	// SpinnerUnpausing is an unpausing spinner
	SpinnerUnpausing
)

func (s SpinnerStatus) String() string {
	switch s {
	case SpinnerStopped:
		return "stopped"
	case SpinnerStarting:
		return "starting"
	case SpinnerRunning:
		return "running"
	case SpinnerStopping:
		return "stopping"
	case SpinnerPausing:
		return "pausing"
	case SpinnerPaused:
		return "paused"
	case SpinnerUnpausing:
		return "unpausing"
	default:
		return fmt.Sprintf("unknown (%d)", s)
	}
}

// Status returns the current status of the spinner. The returned value is of
// type SpinnerStatus, which can be compared against the exported Spinner*
// package-level constants (e.g., SpinnerRunning).
func (s *Spinner) Status() SpinnerStatus {
	return SpinnerStatus(atomic.LoadUint32(s.status))
}

// Start begins the spinner on the Writer in the Config provided to New(). Only
// possible error is if the spinner is already runninng.
func (s *Spinner) Start() error {
	// move us to the starting state
	if !atomic.CompareAndSwapUint32(s.status, statusStopped, statusStarting) {
		return errors.New("spinner already running or shutting down")
	}

	// we now have atomic guarantees of no other goroutines starting or running

	s.mu.Lock()

	s.frequencyUpdateCh = make(chan time.Duration, 4)
	s.dataUpdateCh, s.cancelCh = make(chan struct{}, 1), make(chan struct{}, 1)

	if s.isNotTTY {
		// hack to prevent the animation from running if not a TTY
		s.frequency = time.Duration(math.MaxInt64)
	}

	s.mu.Unlock()

	// because of the atomic swap above, we know it's safe to mutate these
	// values outside of mutex
	s.doneCh = make(chan struct{})
	s.pauseCh = make(chan struct{}) // unbuffered since we want this to be synchronous

	go s.painter(s.cancelCh, s.dataUpdateCh, s.pauseCh, s.doneCh, s.frequencyUpdateCh)

	// move us to the running state
	if !atomic.CompareAndSwapUint32(s.status, statusStarting, statusRunning) {
		panic("atomic invariant encountered")
	}

	return nil
}

// Pause puts the spinner in a state where it no longer animates or renders
// updates to data. This function blocks until the spinner's internal painting
// goroutine enters a paused state.
//
// If you want to make a few configuration changes and have them to appear at
// the same time, like changing the suffix, message, and color, you can Pause()
// the spinner first and then Unpause() after making the changes.
//
// If the spinner is not running (stopped, paused, or in transition to another
// state) this returns an error.
func (s *Spinner) Pause() error {
	if !atomic.CompareAndSwapUint32(s.status, statusRunning, statusPausing) {
		return errors.New("spinner not running")
	}

	// set up the channels the painter will use
	s.unpauseCh, s.unpausedCh = make(chan struct{}), make(chan struct{})

	// inform the painter to pause as a blocking send
	s.pauseCh <- struct{}{}

	if !atomic.CompareAndSwapUint32(s.status, statusPausing, statusPaused) {
		panic("atomic invariant encountered")
	}

	return nil
}

// Unpause returns the spinner back to a running state after pausing. See
// Pause() documentation for more detail. This function blocks until the
// spinner's internal painting goroutine acknowledges the request to unpause.
//
// If the spinner is not paused this returns an error.
func (s *Spinner) Unpause() error {
	if !atomic.CompareAndSwapUint32(s.status, statusPaused, statusUnpausing) {
		return errors.New("spinner not paused")
	}

	s.unpause()

	if !atomic.CompareAndSwapUint32(s.status, statusUnpausing, statusRunning) {
		panic("atomic invariant encountered")
	}

	return nil
}

func (s *Spinner) unpause() {
	// tell the painter to unpause
	close(s.unpauseCh)

	// wait for the painter to signal it will continue
	<-s.unpausedCh

	// clear the no longer needed channels
	s.unpauseCh = nil
	s.unpausedCh = nil
}

// Stop disables the spinner, and prints the StopCharacter with the StopMessage
// using the StopColors. This blocks until the stopped message is printed. Only
// possible error is if the spinner is not running.
func (s *Spinner) Stop() error {
	return s.stop(false)
}

// StopFail disables the spinner, and prints the StopFailCharacter with the
// StopFailMessage using the StopFailColors. This blocks until the stopped
// message is printed. Only possible error is if the spinner is not running.
func (s *Spinner) StopFail() error {
	return s.stop(true)
}

func (s *Spinner) stop(fail bool) error {
	// move us to a stopping state to protect against concurrent Stop() calls
	wasRunning := atomic.CompareAndSwapUint32(s.status, statusRunning, statusStopping)
	wasPaused := atomic.CompareAndSwapUint32(s.status, statusPaused, statusStopping)

	if !wasRunning && !wasPaused {
		return errors.New("spinner not running or paused")
	}

	// we now have an atomic guarantees of no other threads invoking state changes

	if !fail {
		// this tells the painter to print the StopMessage and not the
		// StopFailMessage
		s.cancelCh <- struct{}{}
	}

	close(s.cancelCh)

	if wasPaused {
		s.unpause()
	}

	// wait for the painter to stop
	<-s.doneCh

	s.mu.Lock()

	s.dataUpdateCh = make(chan struct{})           // prevent panic() in various setter methods
	s.frequencyUpdateCh = make(chan time.Duration) // prevent panic() in .Frequency()

	s.mu.Unlock()

	// because of atomic swaps and channel receive above we know it's
	// safe to mutate these fields outside of the mutex
	s.index = 0
	s.cancelCh = nil
	s.doneCh = nil
	s.pauseCh = nil

	// move us to the stopped state
	if !atomic.CompareAndSwapUint32(s.status, statusStopping, statusStopped) {
		panic("atomic invariant encountered")
	}

	return nil
}

// handleFrequencyUpdate is for when the frequency was changed. This tries to
// see if we should fire the timer now, or change its current duration to match
// the new duration.
func handleFrequencyUpdate(newFrequency time.Duration, timer *time.Timer, lastTick time.Time) {
	// if timer fired, drain the channel
	if !timer.Stop() {
	timerLoop:
		for {
			select {
			case <-timer.C:
			default:
				break timerLoop
			}
		}
	}

	timeSince := time.Since(lastTick)

	// if we've exceeded the new delay trigger timer immediately
	if timeSince >= newFrequency {
		timer.Reset(0)
		return
	}

	timer.Reset(newFrequency - timeSince)
}

func (s *Spinner) painter(cancel, dataUpdate, pause <-chan struct{}, done chan<- struct{}, frequencyUpdate <-chan time.Duration) {
	timer := time.NewTimer(0)
	var lastTick time.Time

	for {
		select {
		case <-timer.C:
			lastTick = time.Now()

			s.paintUpdate(timer, true)

		case <-pause:
			<-s.unpauseCh
			close(s.unpausedCh)

		case <-dataUpdate:
			// if this is not a TTY: animate the spinner on the data update
			s.paintUpdate(timer, s.isNotTTY)

		case frequency := <-frequencyUpdate:
			handleFrequencyUpdate(frequency, timer, lastTick)

		case _, ok := <-cancel:
			defer close(done)

			timer.Stop()

			s.paintStop(ok)

			return
		}
	}
}

func (s *Spinner) paintUpdate(timer *time.Timer, animate bool) {
	s.mu.Lock()

	p := s.prefix
	m := s.message
	suf := s.suffix
	mw := s.maxWidth
	cFn := s.colorFn
	d := s.frequency
	index := s.index

	if animate {
		s.index++

		if s.index == len(s.chars) {
			s.index = 0
		}
	} else {
		// for data updates use the last spinner char
		index--

		if index < 0 {
			index = len(s.chars) - 1
		}
	}

	c := s.chars[index]

	s.mu.Unlock()

	defer s.buffer.Reset()

	if !s.isDumbTerm {
		if err := erase(s.buffer); err != nil {
			panic(fmt.Sprintf("failed to erase line: %v", err))
		}

		if s.cursorHidden {
			if err := hideCursor(s.buffer); err != nil {
				panic(fmt.Sprintf("failed to hide cursor: %v", err))
			}
		}

		if _, err := paint(s.buffer, mw, c, p, m, suf, s.suffixAutoColon, s.colorAll, s.spinnerAtEnd, false, s.isNotTTY, cFn); err != nil {
			panic(fmt.Sprintf("failed to paint line: %v", err))
		}
	} else {
		if err := s.eraseDumbTerm(s.buffer); err != nil {
			panic(fmt.Sprintf("failed to erase line: %v", err))
		}

		n, err := paint(s.buffer, mw, c, p, m, suf, s.suffixAutoColon, false, s.spinnerAtEnd, false, s.isNotTTY, fmt.Sprintf)
		if err != nil {
			panic(fmt.Sprintf("failed to paint line: %v", err))
		}

		s.lastPrintLen = n
	}

	if s.buffer.Len() > 0 {
		if _, err := s.writer.Write(s.buffer.Bytes()); err != nil {
			panic(fmt.Sprintf("failed to output buffer to writer: %v", err))
		}
	}

	if animate {
		timer.Reset(d)
	}
}

func (s *Spinner) paintStop(chanOk bool) {
	var m string
	var c character
	var cFn func(format string, a ...interface{}) string

	s.mu.Lock()

	if chanOk {
		c = s.stopChar
		cFn = s.stopColorFn
		m = s.stopMsg
	} else {
		c = s.stopFailChar
		cFn = s.stopFailColorFn
		m = s.stopFailMsg
	}

	p := s.prefix
	suf := s.suffix
	mw := s.maxWidth

	s.mu.Unlock()

	defer s.buffer.Reset()

	if !s.isDumbTerm {
		if err := erase(s.buffer); err != nil {
			panic(fmt.Sprintf("failed to erase line: %v", err))
		}

		if s.cursorHidden {
			if err := unhideCursor(s.buffer); err != nil {
				panic(fmt.Sprintf("failed to hide cursor: %v", err))
			}
		}

		if c.Size > 0 || len(m) > 0 {
			// paint the line with a newline as it's the final line
			if _, err := paint(s.buffer, mw, c, p, m, suf, s.suffixAutoColon, s.colorAll, s.spinnerAtEnd, true, s.isNotTTY, cFn); err != nil {
				panic(fmt.Sprintf("failed to paint line: %v", err))
			}
		}
	} else {
		if err := s.eraseDumbTerm(s.buffer); err != nil {
			panic(fmt.Sprintf("failed to erase line: %v", err))
		}

		if c.Size > 0 || len(m) > 0 {
			if _, err := paint(s.buffer, mw, c, p, m, suf, s.suffixAutoColon, false, s.spinnerAtEnd, true, s.isNotTTY, fmt.Sprintf); err != nil {
				panic(fmt.Sprintf("failed to paint line: %v", err))
			}
		}

		s.lastPrintLen = 0
	}

	if s.buffer.Len() > 0 {
		if _, err := s.writer.Write(s.buffer.Bytes()); err != nil {
			panic(fmt.Sprintf("failed to output buffer to writer: %v", err))
		}
	}
}

// erase clears the line
func erase(w io.Writer) error {
	_, err := fmt.Fprint(w, "\r\033[K\r")
	return err
}

// eraseDumbTerm clears the line on dumb terminals
func (s *Spinner) eraseDumbTerm(w io.Writer) error {
	if s.isNotTTY {
		return nil
	}

	clear := "\r" + strings.Repeat(" ", s.lastPrintLen) + "\r"

	_, err := fmt.Fprint(w, clear)
	return err
}

func hideCursor(w io.Writer) error {
	_, err := fmt.Fprint(w, "\r\033[?25l\r")
	return err
}

func unhideCursor(w io.Writer) error {
	_, err := fmt.Fprint(w, "\r\033[?25h\r")
	return err
}

// padChar pads the spinner character so suffix / message offset from left is
// consistent
func padChar(char character, maxWidth int) string {
	padSize := maxWidth - char.Size
	return char.Value + strings.Repeat(" ", padSize)
}

// paint writes a single line to the w, using the provided character, message,
// and color function
func paint(w io.Writer, maxWidth int, char character, prefix, message, suffix string, suffixAutoColon, colorAll, spinnerAtEnd, finalPaint, notTTY bool, colorFn func(format string, a ...interface{}) string) (int, error) {
	var output string

	switch char.Size {
	case 0:
		if colorAll {
			output = colorFn(message)
			break
		}

		output = message

	default:
		c := padChar(char, maxWidth)

		if spinnerAtEnd {
			if colorAll {
				output = colorFn("%s%s%s%s", message, prefix, c, suffix)
				break
			}

			output = fmt.Sprintf("%s%s%s%s", message, prefix, colorFn(c), suffix)
			break
		}

		if suffixAutoColon { // also implicitly !spinnerAtEnd
			if len(suffix) > 0 && len(message) > 0 && message != "\n" {
				suffix += ": "
			}
		}

		if colorAll {
			output = colorFn("%s%s%s%s", prefix, c, suffix, message)
			break
		}

		output = fmt.Sprintf("%s%s%s%s", prefix, colorFn(c), suffix, message)
	}

	if finalPaint || notTTY {
		output += "\n"
	}

	return fmt.Fprint(w, output)
}

// Frequency updates the frequency of the spinner being animated.
func (s *Spinner) Frequency(d time.Duration) error {
	if d < 1 {
		return errors.New("duration must be greater than 0")
	}

	if s.isNotTTY {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.frequency = d

	// non-blocking notification
	select {
	case s.frequencyUpdateCh <- d:
	default:
	}

	return nil
}

// Prefix updates the Prefix before the spinner character.
func (s *Spinner) Prefix(prefix string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.prefix = prefix

	s.notifyDataChange()
}

// Suffix updates the Suffix printed after the spinner character and before the
// message. It's recommended that this start with an empty space.
func (s *Spinner) Suffix(suffix string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.suffix = suffix

	s.notifyDataChange()
}

// Message updates the Message displayed after the suffix.
func (s *Spinner) Message(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.message = message

	s.notifyDataChange()
}

// Colors updates the github.com/fatih/colors for printing the spinner line.
// ColorAll config parameter controls whether only the spinner character is
// printed with these colors, or the whole line.
//
// StopColors() is the method to control the colors in the stop message.
func (s *Spinner) Colors(colors ...string) error {
	colorFn, err := colorFunc(colors...)
	if err != nil {
		return fmt.Errorf("failed to build color function: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.colorFn = colorFn

	s.notifyDataChange()

	return nil
}

// StopMessage updates the Message used when Stop() is called.
func (s *Spinner) StopMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopMsg = message

	s.notifyDataChange()
}

// StopColors updates the colors used for the stop message. See Colors() method
// documentation for more context.
//
// StopFailColors() is the method to control the colors in the failed stop
// message.
func (s *Spinner) StopColors(colors ...string) error {
	colorFn, err := colorFunc(colors...)
	if err != nil {
		return fmt.Errorf("failed to build stop color function: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopColorFn = colorFn

	s.notifyDataChange()

	return nil
}

// StopCharacter sets the single "character" to use for the spinner when
// stopping. Recommended character is ✓.
func (s *Spinner) StopCharacter(char string) {
	n := runewidth.StringWidth(char)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopChar = character{Value: char, Size: n}

	if n > s.maxWidth {
		s.maxWidth = n
	}

	s.notifyDataChange()
}

// StopFailMessage updates the Message used when StopFail() is called.
func (s *Spinner) StopFailMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopFailMsg = message

	s.notifyDataChange()
}

// StopFailColors updates the colors used for the StopFail message. See Colors() method
// documentation for more context.
func (s *Spinner) StopFailColors(colors ...string) error {
	colorFn, err := colorFunc(colors...)
	if err != nil {
		return fmt.Errorf("failed to build stop fail color function: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopFailColorFn = colorFn

	s.notifyDataChange()

	return nil
}

// StopFailCharacter sets the single "character" to use for the spinner when
// stopping for a failure. Recommended character is ✗.
func (s *Spinner) StopFailCharacter(char string) {
	n := runewidth.StringWidth(char)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopFailChar = character{Value: char, Size: n}

	if n > s.maxWidth {
		s.maxWidth = n
	}

	s.notifyDataChange()
}

// CharSet updates the set of characters (strings) to use for the spinner. You
// can provide your own, or use one from the yacspin.CharSets variable.
//
// The character sets available in the CharSets variable are from the
// https://github.com/briandowns/spinner project.
func (s *Spinner) CharSet(cs []string) error {
	if len(cs) == 0 {
		return errors.New("failed to set character set:  must provide at least one string")
	}

	chars, mw := setToCharSlice(cs)
	s.mu.Lock()
	defer s.mu.Unlock()

	if n := s.stopChar.Size; n > mw {
		mw = s.stopChar.Size
	}

	if n := s.stopFailChar.Size; n > mw {
		mw = n
	}

	s.chars = chars
	s.maxWidth = mw
	s.index = 0

	return nil
}

// Reverse flips the character set order of the spinner characters.
func (s *Spinner) Reverse() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, j := 0, len(s.chars)-1; i < j; {
		s.chars[i], s.chars[j] = s.chars[j], s.chars[i]
		i++
		j--
	}

	s.index = 0
}

func uint32Ptr(u uint32) *uint32 { return &u }
