// Package yacspin provides Yet Another CLi Spinner for Go, taking inspiration
// (and some utility code) from the https://github.com/briandowns/spinner
// project. Specifically this project borrows the default character sets, and
// color mappings to github.com/fatih/color colors, from that project.
//
// This also supports an alternate mode of operation for Winodws OS and dumb
// terminals. This is discovered automatically when creating the spinner.
//
// Within the yacspin package there are some default spinners stored in the
// yacspin.CharSets variable, but you can also provide your own. There is also a
// list of known colors in the yacspin.ValidColors variable.
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
package yacspin

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/pkg/errors"
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
	// Note: This is a required value (cannot be 0)
	Frequency time.Duration

	// Delay is deprecated by the Frequency configuration field, with Frequency
	// taking precedent if both are present.
	Delay time.Duration

	// Writer is the place where we are outputting the spinner, and can't be
	// changed on the fly. If omitted, this defaults to os.Stdout.
	Writer io.Writer

	// HideCursor describes whether the cursor should be hidden by the spinner.
	// If it is hidden, it will be restored when the spinner stops. This can't
	// be changed on the fly.
	HideCursor bool

	// ColorAll describes whether to color everything (all) or just the spinner
	// character(s). This cannot be changed.
	ColorAll bool

	// Colors are the colors used for the different printed messages. This
	// respects the ColorAll field.
	Colors []string

	// CharSet is the list of characters to iterate through to draw the spinner.
	CharSet []string

	// Prefix is the string printed immediately before the spinner.
	Prefix string

	// Suffix is the string printed immediately after the spinner. It's
	// recommended that this string starts with an space ` ` character.
	Suffix string

	// SuffixAutoColon configures whether the spinner adds a colon after the
	// suffix automatically. If there is a message, a colon followed by a space
	// is added to the suffix. Otherwise, if there is no message the colon is
	// omitted.
	SuffixAutoColon bool

	// Message is the string printed after the suffix. If a suffix is present,
	// `: ` is appended to the suffix before printing the message. It results in
	// a message like:
	//
	// <prefix><spinner><suffix>: <message>
	Message string

	// StopMessage is the message used when Stop() is called.
	StopMessage string

	// StopCharacter is spinner character used when Stop() is called.
	// Recommended character is ✓.
	StopCharacter string

	// StopColors are the colors used for the Stop() printed line. This respects
	// the ColorAll field.
	StopColors []string

	// StopFailMessage is the message used when StopFail() is called.
	StopFailMessage string

	// StopFailCharacter is the spinner character used when StopFail() is called.
	// Recommended character is ✗.
	StopFailCharacter string

	// StopFailColors are the colors used for the StopFail() printed line. This
	// respects the ColorAll field.
	StopFailColors []string
}

// Spinner is the struct type representing a spinner. It's configured via the
// Config type, and controlled via its methods. Some configuration can also be
// updated via methods.
//
// Note: You need to use New() to construct a *Spinner.
type Spinner struct {
	writer          io.Writer
	colorAll        bool
	cursorHidden    bool
	suffixAutoColon bool
	isDumbTerm      bool

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

// New creates a new unstarted spinner.
func New(cfg Config) (*Spinner, error) {
	if cfg.Delay > 0 && cfg.Frequency == 0 {
		cfg.Frequency = cfg.Delay
	}

	if cfg.Frequency < 1 {
		return nil, errors.New("cfg.Frequency must be greater than 0")
	}

	s := &Spinner{
		mu:                &sync.Mutex{},
		frequency:         cfg.Frequency,
		status:            uint32Ptr(0),
		frequencyUpdateCh: make(chan time.Duration),
		dataUpdateCh:      make(chan struct{}),

		colorAll:        cfg.ColorAll,
		cursorHidden:    cfg.HideCursor,
		suffixAutoColon: cfg.SuffixAutoColon,
		isDumbTerm:      os.Getenv("TERM") == "dumb" || runtime.GOOS == "windows",
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

	if cfg.Writer == nil {
		cfg.Writer = os.Stdout
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

// Active is deprecated and will be removed in a future release. It was replaced
// by the Status() method.
//
// Active returns whether the spinner is active. Active means the spinner is
// either starting or running.
func (s *Spinner) Active() bool {
	v := atomic.LoadUint32(s.status)
	return v == 1 || v == 2
}

// SpinnerStatus describes the status of the spinner. See the possible constant
// values.
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

	// SpinnerUnpausing is a unpausing spinner
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

// Status returns the current status of the internal state machine. Returned
// value is of type SpinnerStatus which has package constants available.
func (s *Spinner) Status() SpinnerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == nil {
		panic("status field is nil")
	}

	return SpinnerStatus(atomic.LoadUint32(s.status))
}

// Start begins the spinner on the Writer in the Config provided to New(). Onnly
// possible error is if the spinner is already runninng.
func (s *Spinner) Start() error {
	// move us to the starting state
	if !atomic.CompareAndSwapUint32(s.status, statusStopped, statusStarting) {
		return errors.New("spinner already running or shutting down")
	}

	// we now have atomic guarantees of no other threads starting or running

	s.mu.Lock()

	s.frequencyUpdateCh = make(chan time.Duration, 1)
	s.dataUpdateCh, s.cancelCh = make(chan struct{}, 1), make(chan struct{}, 1)

	s.mu.Unlock()

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
// updates to data. This function blocks until the spinner's internal goroutine
// enters a paused state.
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

	// inform the painter to pause
	s.pauseCh <- struct{}{}

	if !atomic.CompareAndSwapUint32(s.status, statusPausing, statusPaused) {
		panic("atomic invariant encountered")
	}

	return nil
}

// Unpause returns the spinner back to a running state after pausing. See
// Pause() documentation for more detail. This function blocks until the
// spinner's internal goroutine acknowledges the request to unpause.
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

			s.paintUpdate(timer, false)

		case <-pause:
			<-s.unpauseCh
			close(s.unpausedCh)

		case <-dataUpdate:
			s.paintUpdate(timer, true)

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

func (s *Spinner) paintUpdate(timer *time.Timer, dataUpdate bool) {
	s.mu.Lock()

	p := s.prefix
	m := s.message
	suf := s.suffix
	mw := s.maxWidth
	cFn := s.colorFn
	d := s.frequency
	index := s.index

	if !dataUpdate {
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

	if !s.isDumbTerm {
		if err := s.erase(); err != nil {
			panic(fmt.Sprintf("failed to erase line: %v", err))
		}

		if s.cursorHidden {
			if err := s.hideCursor(); err != nil {
				panic(fmt.Sprintf("failed to hide cursor: %v", err))
			}
		}

		if _, err := paint(s.writer, mw, c, p, m, suf, s.suffixAutoColon, s.colorAll, cFn); err != nil {
			panic(fmt.Sprintf("failed to paint line: %v", err))
		}
	} else {
		if err := s.eraseWindows(); err != nil {
			panic(fmt.Sprintf("failed to erase line: %v", err))
		}

		n, err := paint(s.writer, mw, c, p, m, suf, s.suffixAutoColon, false, fmt.Sprintf)

		if err != nil {
			panic(fmt.Sprintf("failed to paint line: %v", err))
		}

		s.lastPrintLen = n
	}

	if !dataUpdate {
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

	if !s.isDumbTerm {
		if err := s.erase(); err != nil {
			panic(fmt.Sprintf("failed to erase line: %v", err))
		}

		if s.cursorHidden {
			if err := s.unhideCursor(); err != nil {
				panic(fmt.Sprintf("failed to hide cursor: %v", err))
			}
		}

		if c.Size == 0 && len(m) == 0 {
			return
		}

		// paint the line with a newline as it's the final line
		if _, err := paint(s.writer, mw, c, p, m+"\n", suf, s.suffixAutoColon, s.colorAll, cFn); err != nil {
			panic(fmt.Sprintf("failed to paint line: %v", err))
		}

	} else {
		if err := s.eraseWindows(); err != nil {
			panic(fmt.Sprintf("failed to erase line: %v", err))
		}

		if c.Size == 0 && len(m) == 0 {
			return
		}

		if _, err := paint(s.writer, mw, c, p, m+"\n", suf, s.suffixAutoColon, false, fmt.Sprintf); err != nil {
			panic(fmt.Sprintf("failed to paint line: %v", err))
		}

		s.lastPrintLen = 0
	}
}

// erase clears the line
func (s *Spinner) erase() error {
	_, err := fmt.Fprint(s.writer, "\r\033[K\r")
	return err
}

// eraseWindows clears the line on Windows
func (s *Spinner) eraseWindows() error {
	clear := "\r" + strings.Repeat(" ", s.lastPrintLen) + "\r"

	_, err := fmt.Fprint(s.writer, clear)
	return err
}

func (s *Spinner) hideCursor() error {
	_, err := fmt.Fprint(s.writer, "\r\033[?25l\r")
	return err
}

func (s *Spinner) unhideCursor() error {
	_, err := fmt.Fprint(s.writer, "\r\033[?25h\r")
	return err
}

// padChar pads the spinner character so suffix / message offset from left is
// consistent
func padChar(char character, maxWidth int) string {
	padSize := maxWidth - char.Size
	return char.Value + strings.Repeat(" ", padSize)
}

// paint writes a single line to the s.writer, using the provided character,
// message, and color function
func paint(w io.Writer, maxWidth int, char character, prefix, message, suffix string, suffixAutoColon, colorAll bool, colorFn func(format string, a ...interface{}) string) (int, error) {
	if char.Size == 0 {
		if colorAll {
			return fmt.Fprint(w, colorFn(message))
		}

		return fmt.Fprint(w, message)
	}

	c := padChar(char, maxWidth)

	if suffixAutoColon {
		if len(suffix) > 0 && len(message) > 0 && message != "\n" {
			suffix += ": "
		}
	}

	if colorAll {
		return fmt.Fprint(w, colorFn("%s%s%s%s", prefix, c, suffix, message))
	}

	return fmt.Fprintf(w, "%s%s%s%s", prefix, colorFn(c), suffix, message)
}

// Delay is deprecated in favor of Frequency.
func (s *Spinner) Delay(d time.Duration) error {
	return s.Frequency(d)
}

// Frequency updates the frequency of the spinner being animated.
func (s *Spinner) Frequency(d time.Duration) error {
	if d < 1 {
		return errors.New("duration must be greater than 0")
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

// Suffix updates the Suffix after the spinner character. It's recommended that
// this start with an empty space.
func (s *Spinner) Suffix(suffix string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.suffix = suffix

	s.notifyDataChange()
}

// Message updates the Message displayed after he suffix.
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
		return errors.Wrapf(err, "failed to build color function")
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
func (s *Spinner) StopColors(colors ...string) error {
	colorFn, err := colorFunc(colors...)
	if err != nil {
		return errors.Wrapf(err, "failed to build stop color function")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopColorFn = colorFn

	s.notifyDataChange()

	return nil
}

// StopCharacter sets the single "character" to use for the spinner. Recommended
// character is ✓.
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
		return errors.Wrapf(err, "failed to build stop fail color function")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopFailColorFn = colorFn

	s.notifyDataChange()

	return nil
}

// StopFailCharacter sets the single "character" to use for the spinner. Recommended
// character is ✗.
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
// can provide your own, or use one from the CharSets variable.
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
