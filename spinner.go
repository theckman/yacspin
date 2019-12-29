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

func atomicString(v *atomic.Value) string {
	av := v.Load()
	if av == nil {
		return ""
	}

	s, ok := av.(string)
	if !ok {
		return ""
	}

	return s
}

func atomicDuration(u *int64) time.Duration {
	i64 := atomic.LoadInt64(u)

	return time.Duration(i64)
}

func atomicColorFn(v *atomic.Value) func(format string, a ...interface{}) string {
	av := v.Load()
	if av == nil {
		return func(format string, a ...interface{}) string { return "" }
	}

	fn, ok := av.(func(format string, a ...interface{}) string)
	if !ok {
		return func(format string, a ...interface{}) string { return "" }
	}

	return fn
}

func atomicCharacter(v *atomic.Value) character {
	av := v.Load()
	if av == nil {
		return character{}
	}

	c, ok := av.(character)
	if !ok {
		return character{}
	}

	return c
}

type character struct {
	value string
	size  int
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
			value: s,
			size:  n,
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
	// Recommended character is `✓`.
	StopCharacter string

	// StopColors are the colors used for the Stop() printed line. This respects
	// the ColorAll field.
	StopColors []string

	// StopFailMessage is the message used when StopFail() is called.
	StopFailMessage string

	// StopFailCharacter is the spinner character used when StopFail() is called.
	// Recommended character is `✗`.
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
	writer   io.Writer
	colorAll bool

	mu       *sync.RWMutex
	chars    []character
	maxWidth int
	index    int

	active   *uint32
	cancelCh chan struct{} // send: Stop(), close: StopFail(); both stop painter
	sigCh    chan struct{}

	delayDuration *int64 // to allow atomic updates

	colorFn *atomic.Value
	prefix  *atomic.Value
	suffix  *atomic.Value
	message *atomic.Value

	stopMsg     *atomic.Value
	stopChar    *atomic.Value
	stopColorFn *atomic.Value

	stopFailMsg     *atomic.Value
	stopFailChar    *atomic.Value
	stopFailColorFn *atomic.Value
}

// New creates a new unstarted spinner.
func New(cfg Config) (*Spinner, error) {
	if cfg.Delay < 1 {
		return nil, errors.New("cfg.Delay must be greater than 0")
	}

	s := &Spinner{
		mu:            &sync.RWMutex{},
		delayDuration: int64Ptr(int64(cfg.Delay)),
		active:        uint32Ptr(0),
		colorAll:      cfg.ColorAll,

		colorFn: &atomic.Value{},
		prefix:  &atomic.Value{},
		suffix:  &atomic.Value{},
		message: &atomic.Value{},

		stopMsg:     &atomic.Value{},
		stopChar:    &atomic.Value{},
		stopColorFn: &atomic.Value{},

		stopFailMsg:     &atomic.Value{},
		stopFailChar:    &atomic.Value{},
		stopFailColorFn: &atomic.Value{},
	}

	colorFn, err := colorFunc(cfg.Colors...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to build color function")
	}

	s.colorFn.Store(colorFn)

	stopColorFn, err := colorFunc(cfg.StopColors...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to build successs color function")
	}

	s.stopColorFn.Store(stopColorFn)

	stopFailColorFn, err := colorFunc(cfg.StopFailColors...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to build fail color function")
	}

	s.stopFailColorFn.Store(stopFailColorFn)

	if cfg.Writer == nil {
		cfg.Writer = os.Stdout
	}

	s.writer = cfg.Writer

	if len(cfg.CharSet) == 0 {
		cfg.CharSet = CharSets[9]
	}

	s.chars, s.maxWidth = setToCharSlice(cfg.CharSet)

	if len(cfg.Prefix) > 0 {
		s.prefix.Store(cfg.Prefix)
	}

	if len(cfg.Suffix) > 0 {
		s.suffix.Store(cfg.Suffix)
	}

	if len(cfg.Message) > 0 {
		s.message.Store(cfg.Message)
	}

	if len(cfg.StopMessage) > 0 {
		s.stopMsg.Store(cfg.StopMessage)
	}

	if len(cfg.StopCharacter) > 0 {
		n := runewidth.StringWidth(cfg.StopCharacter)

		s.stopChar.Store(character{value: cfg.StopCharacter, size: n})
	}

	if len(cfg.StopFailMessage) > 0 {
		s.stopFailMsg.Store(cfg.StopFailMessage)
	}

	if len(cfg.StopFailCharacter) > 0 {
		n := runewidth.StringWidth(cfg.StopFailCharacter)

		s.stopFailChar.Store(character{value: cfg.StopFailCharacter, size: n})
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

	cancel, sig := make(chan struct{}, 1), make(chan struct{})
	s.cancelCh = cancel
	s.sigCh = sig

	go s.painter(cancel, sig)

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
	<-s.sigCh

	s.index = 0
	s.cancelCh = nil
	s.sigCh = nil

	// move us to the stopped state
	a = atomic.CompareAndSwapUint32(s.active, 3, 0)
	if !a {
		panic("atomic invariant encountered")
	}

	return nil
}

func (s *Spinner) painter(cancel, sig chan struct{}) {
	for {
		select {
		case _, ok := <-cancel:
			defer close(sig)

			if err := s.erase(); err != nil {
				panic(fmt.Sprintf("failed to erase line: %v", err))
			}

			var m string
			var c character
			var cFn func(format string, a ...interface{}) string

			if ok {
				c = atomicCharacter(s.stopChar)
				cFn = atomicColorFn(s.stopColorFn)
				m = atomicString(s.stopMsg)
			} else {
				c = atomicCharacter(s.stopFailChar)
				cFn = atomicColorFn(s.stopFailColorFn)
				m = atomicString(s.stopFailMsg)
			}

			if c.size == 0 && len(m) == 0 {
				return
			}

			// paint the line with a newline as it's the final line
			if err := s.paint(c, m+"\n", cFn); err != nil {
				panic(fmt.Sprintf("failed to paint stop line: %v", err))
			}

			return

		default:
			s.mu.Lock()

			c := s.chars[s.index]
			s.index++

			if s.index == len(s.chars) {
				s.index = 0
			}

			s.mu.Unlock()

			if err := s.erase(); err != nil {
				panic(fmt.Sprintf("failed to erase line: %v", err))
			}

			if err := s.paint(c, atomicString(s.message), atomicColorFn(s.colorFn)); err != nil {
				panic(fmt.Sprintf("failed to paint line: %v", err))
			}

			time.Sleep(atomicDuration(s.delayDuration))
		}

	}
}

// erase clears the line
func (s *Spinner) erase() error {
	_, err := fmt.Fprint(s.writer, "\r\033[K\r")
	return err
}

// padChar pads the spinner character so suffix / message offset from left is
// consistent
func padChar(char character, maxWidth int) string {
	padSize := maxWidth - char.size
	return char.value + strings.Repeat(" ", padSize)
}

// paint writes a single line to the s.writer, using the provided character,
// message, and color function
func (s *Spinner) paint(char character, message string, colorFn func(format string, a ...interface{}) string) error {
	if char.size == 0 {
		if s.colorAll {
			fmt.Fprint(s.writer, colorFn(message))
		} else {
			fmt.Fprint(s.writer, message)
		}

		return nil
	}

	p, suf := atomicString(s.prefix), atomicString(s.suffix)

	if len(suf) > 0 {
		if len(message) > 0 && message != "\n" {
			suf += ": "
		}
	}

	c := padChar(char, s.maxWidth)

	if s.colorAll {
		fmt.Fprint(s.writer, colorFn("%s%s%s%s", p, c, suf, message))
	} else {
		c = colorFn(c)
		fmt.Fprintf(s.writer, "%s%s%s%s", p, c, suf, message)
	}

	return nil
}

// Delay updates the Delay between repainting the line.
func (s *Spinner) Delay(d time.Duration) error {
	if d < 1 {
		return errors.New("delay must be greater than 0")
	}

	atomic.StoreInt64(s.delayDuration, int64(d))

	return nil
}

// Prefix updates the Prefix before the spinner character.
func (s *Spinner) Prefix(prefix string) {
	s.prefix.Store(prefix)
}

// Suffix updates the Suffix after the spinner character. It's recommended that
// this start with an empty space.
func (s *Spinner) Suffix(suffix string) {
	s.suffix.Store(suffix)
}

// Message updates the Message displayed after he suffix.
func (s *Spinner) Message(message string) {
	s.message.Store(message)
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

	s.colorFn.Store(colorFn)

	return nil
}

// StopMessage updates the Message used when Stop() is called.
func (s *Spinner) StopMessage(message string) {
	s.stopMsg.Store(message)

}

// StopColors updates the colors used for the stop message. See Colors() method
// documentation for more context.
func (s *Spinner) StopColors(colors ...string) error {
	colorFn, err := colorFunc(colors...)
	if err != nil {
		return errors.Wrapf(err, "failed to build color function")
	}

	s.stopColorFn.Store(colorFn)

	return nil
}

// StopCharacter sets the single "character" to use for the spinner. Recommended
// character is `✓`.
func (s *Spinner) StopCharacter(char string) {
	n := runewidth.StringWidth(char)

	s.stopChar.Store(character{value: char, size: n})
}

// StopFailMessage updates the Message used when StopFail() is called.
func (s *Spinner) StopFailMessage(message string) {
	s.stopFailMsg.Store(message)

}

// StopFailColors updates the colors used for the StopFail message. See Colors() method
// documentation for more context.
func (s *Spinner) StopFailColors(colors ...string) error {
	colorFn, err := colorFunc(colors...)
	if err != nil {
		return errors.Wrapf(err, "failed to build color function")
	}

	s.stopFailColorFn.Store(colorFn)

	return nil
}

// StopFailCharacter sets the single "character" to use for the spinner. Recommended
// character is `✗`.
func (s *Spinner) StopFailCharacter(char string) {
	n := runewidth.StringWidth(char)

	s.stopFailChar.Store(character{value: char, size: n})
}

// CharSet updates the set of characters (strings) to use for the spinner. You
// can provide your own, or use one from the CharSets variable.
//
// The character sets available in the CharSets variable are from the
// https://github.com/briandowns/spinner project.
func (s *Spinner) CharSet(cs []string) {
	chars, mw := setToCharSlice(cs)
	s.mu.Lock()
	defer s.mu.Unlock()

	s.chars = chars
	s.maxWidth = mw
	s.index = 0
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
