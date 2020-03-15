// Package yacspin provides Yet Another CLi Spinner for Go, taking inspiration
// (and some utility code) from the https://github.com/briandowns/spinner
// project. Specifically this project borrows the default character sets, and
// color mappings to github.com/fatih/color colors, from that project.
//
// Within the yacspin package there are some default spinners stored in the
// yacspin.CharSets variable, but you can also provide your own. There is also a
// list of known colors in the yacspin.ValidColors variable.
//
//		cfg := yacspin.Config{
//			Delay:         100 * time.Millisecond,
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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/pkg/errors"
)

func atomicDuration(u *int64) time.Duration {
	i64 := atomic.LoadInt64(u)

	return time.Duration(i64)
}

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
	// Delay specifies how long to way between repainting the line. Optimal
	// value depends on the character set you use.
	//
	// Note: This is a required value (cannot be 0)
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
	writer       io.Writer
	colorAll     bool
	cursorHidden bool

	active        *uint32
	delayDuration *int64        // to allow atomic updates
	cancelCh      chan struct{} // send: Stop(), close: StopFail(); both stop painter
	duCh          chan struct{}
	doneCh        chan struct{}

	mu              *sync.Mutex
	chars           []character
	maxWidth        int
	index           int
	prefix          string
	suffix          string
	message         string
	colorFn         func(format string, a ...interface{}) string
	stopMsg         string
	stopChar        character
	stopColorFn     func(format string, a ...interface{}) string
	stopFailMsg     string
	stopFailChar    character
	stopFailColorFn func(format string, a ...interface{}) string
}

// New creates a new unstarted spinner.
func New(cfg Config) (*Spinner, error) {
	if cfg.Delay < 1 {
		return nil, errors.New("cfg.Delay must be greater than 0")
	}

	s := &Spinner{
		mu:            &sync.Mutex{},
		delayDuration: int64Ptr(int64(cfg.Delay)),
		active:        uint32Ptr(0),
		duCh:          make(chan struct{}, 1),

		colorAll:        cfg.ColorAll,
		cursorHidden:    cfg.HideCursor,
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

// Active returns whether the spinner is active. Active means the spinner is
// either starting or running.
func (s *Spinner) Active() bool {
	v := atomic.LoadUint32(s.active)
	return v == 1 || v == 2
}

// Start begins the spinner on the Writer in the Config provided to New(). Onnly
// possible error is if the spinner is already runninng.
func (s *Spinner) Start() error {
	// move us to the starting state
	if !atomic.CompareAndSwapUint32(s.active, 0, 1) {
		return errors.New("spinner already running or shutting down")
	}

	// we now have atomic guarantees of no other threads starting or running

	s.cancelCh, s.doneCh = make(chan struct{}, 1), make(chan struct{})

	go s.painter(s.cancelCh, s.duCh, s.doneCh)

	// move us to the running state
	if !atomic.CompareAndSwapUint32(s.active, 1, 2) {
		panic("atomic invariant detected")
	}

	return nil
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
	a := atomic.CompareAndSwapUint32(s.active, 2, 3)
	if !a {
		return errors.New("spinner not running or shutting down")
	}

	// we now have an atomic guarantees of no other threads invoking state changes

	if !fail {
		// this tells the painter to print the StopMessage and not the
		// StopFailMessage
		s.cancelCh <- struct{}{}
	}

	close(s.cancelCh)

	// wait for the painter to stop
	<-s.doneCh

	s.index = 0
	s.cancelCh = nil
	s.doneCh = nil

	// move us to the stopped state
	a = atomic.CompareAndSwapUint32(s.active, 3, 0)
	if !a {
		panic("atomic invariant encountered")
	}

	return nil
}

// handleDelayUpdate is for when the delay duration was changed. This tries to
// see if we should fire the timer now, or change its current duration to match
// the new duration.
func (s *Spinner) handleDelayUpdate(timer *time.Timer, lastTick time.Time) {
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
	delay := atomicDuration(s.delayDuration)

	// if we've exceeded the new delay trigger timer immediately
	if timeSince >= delay {
		timer.Reset(0)
		return
	}

	timer.Reset(delay - timeSince)
}

func (s *Spinner) painter(cancel, delayUpdate <-chan struct{}, done chan<- struct{}) {
	timer := time.NewTimer(0)
	var lastTick time.Time

	for {
		select {
		case <-timer.C:
			lastTick = time.Now()

			s.paintUpdate(timer)

		case <-delayUpdate:
			s.handleDelayUpdate(timer, lastTick)

		case _, ok := <-cancel:
			defer close(done)

			timer.Stop()

			s.paintStop(ok)

			return
		}

	}
}

func (s *Spinner) paintUpdate(timer *time.Timer) {
	s.mu.Lock()

	p := s.prefix
	m := s.message
	suf := s.suffix
	mw := s.maxWidth
	cFn := s.colorFn
	c := s.chars[s.index]

	s.index++
	if s.index == len(s.chars) {
		s.index = 0
	}

	s.mu.Unlock()

	if err := s.erase(); err != nil {
		panic(fmt.Sprintf("failed to erase line: %v", err))
	}

	if s.cursorHidden {
		if err := s.hideCursor(); err != nil {
			panic(fmt.Sprintf("failed to hide cursor: %v", err))
		}
	}

	if err := paint(s.writer, mw, c, p, m, suf, s.colorAll, cFn); err != nil {
		panic(fmt.Sprintf("failed to paint line: %v", err))
	}

	timer.Reset(atomicDuration(s.delayDuration))
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

	if err := s.erase(); err != nil {
		panic(fmt.Sprintf("failed to erase line: %v", err))
	}

	if s.cursorHidden {
		if err := s.unhideCursor(); err != nil {
			panic(fmt.Sprintf("failed to unhide cursor: %v", err))
		}
	}

	if c.Size == 0 && len(m) == 0 {
		return
	}

	// paint the line with a newline as it's the final line
	if err := paint(s.writer, mw, c, p, m+"\n", suf, s.colorAll, cFn); err != nil {
		panic(fmt.Sprintf("failed to paint stop line: %v", err))
	}
}

// erase clears the line
func (s *Spinner) erase() error {
	_, err := fmt.Fprint(s.writer, "\r\033[K\r")
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
func paint(w io.Writer, maxWidth int, char character, prefix, message, suffix string, colorAll bool, colorFn func(format string, a ...interface{}) string) error {
	if char.Size == 0 {
		if colorAll {
			fmt.Fprint(w, colorFn(message))
		} else {
			fmt.Fprint(w, message)
		}

		return nil
	}

	if len(suffix) > 0 {
		if len(message) > 0 && message != "\n" {
			suffix += ": "
		}
	}

	c := padChar(char, maxWidth)

	if colorAll {
		fmt.Fprint(w, colorFn("%s%s%s%s", prefix, c, suffix, message))
	} else {
		fmt.Fprintf(w, "%s%s%s%s", prefix, colorFn(c), suffix, message)
	}

	return nil
}

// Delay updates the Delay between repainting the line.
func (s *Spinner) Delay(d time.Duration) error {
	if d < 1 {
		return errors.New("delay must be greater than 0")
	}

	atomic.StoreInt64(s.delayDuration, int64(d))

	// non-blocking notification
	select {
	case s.duCh <- struct{}{}:
	default:
	}

	return nil
}

// Prefix updates the Prefix before the spinner character.
func (s *Spinner) Prefix(prefix string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.prefix = prefix
}

// Suffix updates the Suffix after the spinner character. It's recommended that
// this start with an empty space.
func (s *Spinner) Suffix(suffix string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.suffix = suffix
}

// Message updates the Message displayed after he suffix.
func (s *Spinner) Message(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.message = message
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

	return nil
}

// StopMessage updates the Message used when Stop() is called.
func (s *Spinner) StopMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopMsg = message
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
}

// StopFailMessage updates the Message used when StopFail() is called.
func (s *Spinner) StopFailMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopFailMsg = message
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

func int64Ptr(i int64) *int64    { return &i }
func uint32Ptr(u uint32) *uint32 { return &u }
