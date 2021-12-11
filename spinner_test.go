package yacspin

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/google/go-cmp/cmp"
	"github.com/mattn/go-runewidth"
)

// testErrCheck looks to see if errContains is a substring of err.Error(). If
// not, this calls t.Fatal(). It also calls t.Fatal() if there was an error, but
// errContains is empty. Returns true if you should continue running the test,
// or false if you should stop the test.
func testErrCheck(t *testing.T, name string, errContains string, err error) bool {
	t.Helper()

	if len(errContains) > 0 {
		if err == nil {
			t.Fatalf("%s error = <nil>, should contain %q", name, errContains)
			return false
		}

		if errStr := err.Error(); !strings.Contains(errStr, errContains) {
			t.Fatalf("%s error = %q, should contain %q", name, errStr, errContains)
			return false
		}

		return false
	}

	if err != nil && len(errContains) == 0 {
		t.Fatalf("%s unexpected error: %v", name, err)
		return false
	}

	return true
}

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		writer   io.Writer
		maxWidth int
		cfg      Config
		charSet  []character
		err      string
	}{
		{
			name:   "empty_config",
			writer: os.Stdout,
			err:    "cfg.Frequency must be greater than 0",
		},
		{
			name:     "config_with_frequency_and_default_writer",
			maxWidth: 1,
			writer:   os.Stdout,
			cfg: Config{
				Frequency: 100 * time.Millisecond,
			},
		},
		{
			name:   "config_with_frequency_and_invalid_colors",
			writer: os.Stdout,
			cfg: Config{
				Frequency: 100 * time.Millisecond,
				Colors:    []string{"invalid"},
			},
			err: "failed to build color function: invalid is not a valid color",
		},
		{
			name:   "config_with_frequency_and_invalid_stopColors",
			writer: os.Stdout,
			cfg: Config{
				Frequency:  100 * time.Millisecond,
				StopColors: []string{"invalid"},
			},
			err: "failed to build stop color function: invalid is not a valid color",
		},
		{
			name:   "config_with_frequency_and_invalid_stopFailColors",
			writer: os.Stdout,
			cfg: Config{
				Frequency:      100 * time.Millisecond,
				StopFailColors: []string{"invalid"},
			},
			err: "failed to build stop fail color function: invalid is not a valid color",
		},
		{
			name:     "full_config",
			writer:   os.Stderr,
			maxWidth: 3,
			cfg: Config{
				Frequency:         100 * time.Millisecond,
				Writer:            os.Stderr,
				HideCursor:        true,
				ColorAll:          true,
				Colors:            []string{"fgYellow"},
				CharSet:           CharSets[59],
				Prefix:            "test prefix: ",
				Suffix:            " test suffix",
				Message:           "test message",
				StopMessage:       "test stop message",
				StopCharacter:     "✓",
				StopColors:        []string{"fgGreen"},
				StopFailMessage:   "test stop fail message",
				StopFailCharacter: "✗",
				StopFailColors:    []string{"fgHiRed"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spinner, err := New(tt.cfg)

			if cont := testErrCheck(t, "New()", tt.err, err); !cont {
				return
			}

			if spinner == nil {
				t.Fatal("spinner is nil")
			}

			if spinner.colorAll != tt.cfg.ColorAll {
				t.Fatalf("spinner.colorAll = %t, want %t", spinner.colorAll, tt.cfg.ColorAll)
			}

			if spinner.cursorHidden != tt.cfg.HideCursor {
				t.Fatalf("spinner.cursorHiddenn = %t, want %t", spinner.cursorHidden, tt.cfg.HideCursor)
			}

			if spinner.mu == nil {
				t.Fatal("spinner.mu is nil")
			}

			if spinner.status == nil {
				t.Fatal("spinner.status is nil")
			}

			if spinner.frequencyUpdateCh == nil {
				t.Fatal("spinner.frequencyUpdateCh is nil")
			}

			if spinner.frequency != tt.cfg.Frequency {
				t.Errorf("spinner.frequency = %s, want %s", spinner.frequency, tt.cfg.Frequency)
			}

			if spinner.writer == nil {
				t.Fatal("spinner.writer is nil")
			}

			if spinner.writer != tt.writer {
				t.Errorf("spinner.writer = %#v, want %#v", spinner.writer, tt.writer)
			}

			if spinner.prefix != tt.cfg.Prefix {
				t.Errorf("spinner.prefix = %q, want %q", spinner.prefix, tt.cfg.Prefix)
			}

			if spinner.suffix != tt.cfg.Suffix {
				t.Errorf("spinner.suffix = %q, want %q", spinner.suffix, tt.cfg.Suffix)
			}

			if spinner.message != tt.cfg.Message {
				t.Errorf("spinner.message = %q, want %q", spinner.message, tt.cfg.Message)
			}

			if spinner.stopMsg != tt.cfg.StopMessage {
				t.Errorf("spinner.stopMsg = %q, want %q", spinner.stopMsg, tt.cfg.StopMessage)
			}

			sc := character{Value: tt.cfg.StopCharacter, Size: runewidth.StringWidth(tt.cfg.StopCharacter)}
			if spinner.stopChar != sc {
				t.Errorf("spinner.stopChar = %#v, want %#v", spinner.stopChar, sc)
			}

			if spinner.stopFailMsg != tt.cfg.StopFailMessage {
				t.Errorf("spinner.stopFailMsg = %q, want %q", spinner.stopFailMsg, tt.cfg.StopFailMessage)
			}

			sfc := character{Value: tt.cfg.StopFailCharacter, Size: runewidth.StringWidth(tt.cfg.StopFailCharacter)}
			if spinner.stopFailChar != sfc {
				t.Errorf("spinner.stopFailChar = %#v, want %#v", spinner.stopFailChar, sfc)
			}

			if spinner.colorFn == nil {
				t.Fatal("spinner.colorFn is nil")
			}

			a := make([]color.Attribute, len(tt.cfg.Colors))

			for i, c := range tt.cfg.Colors {
				ca, ok := colorAttributeMap[c]
				if !ok {
					continue
				}
				a[i] = ca
			}

			tfn := color.New(a...).SprintfFunc()
			gotStr, wantStr := spinner.colorFn("%s: %d", "test string", 42), tfn("%s: %d", "test string", 42)

			if gotStr != wantStr {
				t.Errorf(`spinner.colorFn("%%s: %%d", "test string", 42) = %q, want %q`, gotStr, wantStr)
			}

			if spinner.stopColorFn == nil {
				t.Fatal("spinner.stopColorFn is nil")
			}

			a = make([]color.Attribute, len(tt.cfg.StopColors))

			for i, c := range tt.cfg.StopColors {
				ca, ok := colorAttributeMap[c]
				if !ok {
					continue
				}
				a[i] = ca
			}

			tfn = color.New(a...).SprintfFunc()

			gotStr, wantStr = spinner.stopColorFn("%s: %d", "test string", 42), tfn("%s: %d", "test string", 42)

			if gotStr != wantStr {
				t.Errorf(`spinner.stopColorFn("%%s: %%d", "test string", 42) = %q, want %q`, gotStr, wantStr)
			}

			if spinner.stopFailColorFn == nil {
				t.Fatal("spinner.stopFailColorFn is nil")
			}

			a = make([]color.Attribute, len(tt.cfg.StopFailColors))

			for i, c := range tt.cfg.StopFailColors {
				ca, ok := colorAttributeMap[c]
				if !ok {
					continue
				}
				a[i] = ca
			}

			tfn = color.New(a...).SprintfFunc()

			gotStr, wantStr = spinner.stopFailColorFn("%s: %d", "test string", 42), tfn("%s: %d", "test string", 42)

			if gotStr != wantStr {
				t.Errorf(`spinner.stopFailColorFn("%%s: %%d", "test string", 42) = %q, want %q`, gotStr, wantStr)
			}

			// handle the default value in New()
			if len(tt.cfg.CharSet) == 0 {
				tt.cfg.CharSet = CharSets[9]
			}

			tt.charSet = make([]character, len(tt.cfg.CharSet))

			for i, char := range tt.cfg.CharSet {
				tt.charSet[i] = character{
					Value: char,
					Size:  runewidth.StringWidth(char),
				}
			}

			if diff := cmp.Diff(tt.charSet, spinner.chars); diff != "" {
				t.Fatalf("spinner.chars differs: (-want +got)\n%s", diff)
			}

			if spinner.maxWidth != tt.maxWidth {
				t.Errorf("spinner.maxWidth = %d, want %d", spinner.maxWidth, tt.maxWidth)
			}
		})
	}
}

func TestNew_dumbTerm(t *testing.T) {
	t.Setenv("TERM", "dumb")

	cfg := Config{
		Frequency:     500 * time.Millisecond,
		CharSet:       CharSets[59],
		Suffix:        " backing up database to S3: ",
		Message:       "exporting data to file",
		StopCharacter: "✓",
		StopColors:    []string{"fgGreen"},
		HideCursor:    true,
		ColorAll:      true,
	}

	spinner, err := New(cfg)
	testErrCheck(t, "New()", "", err)

	if !spinner.isDumbTerm {
		t.Fatal("spinner.isDumbTerm = false, want true")
	}
}

func TestSpinner_Status(t *testing.T) {
	tests := []struct {
		name        string
		spinner     *Spinner
		shouldPanic bool
		want        SpinnerStatus
	}{
		{
			name:        "should_panic",
			spinner:     &Spinner{mu: &sync.Mutex{}},
			shouldPanic: true,
		},
		{
			name: "active_status",
			spinner: &Spinner{
				mu:     &sync.Mutex{},
				status: uint32Ptr(statusUnpausing),
			},
			want: SpinnerUnpausing,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var got SpinnerStatus

			panicked := func() (p bool) {
				defer func() {
					if r := recover(); r != nil {
						p = true
					}
				}()

				got = tt.spinner.Status()

				return false
			}()

			if panicked != tt.shouldPanic {
				t.Fatalf("panicked = %t, want %t", panicked, tt.shouldPanic)
			}

			if tt.shouldPanic {
				return
			}

			if got != tt.want {
				t.Fatalf("got = %d, want = %d", got, tt.want)
			}
		})
	}
}

func TestSpinner_notifyDataChange(t *testing.T) {
	tests := []struct {
		name          string
		spinner       *Spinner
		want          bool
		shouldReceive bool
	}{
		{
			name:          "buffered_channel",
			spinner:       &Spinner{dataUpdateCh: make(chan struct{}, 1)},
			want:          true,
			shouldReceive: true,
		},
		{
			name:          "unbuffered_channel",
			spinner:       &Spinner{dataUpdateCh: make(chan struct{})},
			shouldReceive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.spinner.notifyDataChange()

			select {
			case _, got := <-tt.spinner.dataUpdateCh:
				if !tt.shouldReceive {
					t.Fatal("unexpected channel receive")
				}

				if got != tt.want {
					t.Errorf("got = %t, want %t", got, tt.want)
				}
			default:
				if tt.shouldReceive {
					t.Fatal("nothing received over channel")
				}
			}
		})
	}
}

func TestSpinner_Frequency(t *testing.T) {
	tests := []struct {
		name  string
		input time.Duration
		ch    chan time.Duration
		err   string
	}{
		{
			name: "invalid",
			ch:   make(chan time.Duration, 1),
			err:  "duration must be greater than 0",
		},
		{
			name:  "assert_non-blocking",
			input: 42,
			ch:    make(chan time.Duration, 1),
		},
		{
			name:  "assert_notification",
			input: 42,
			ch:    make(chan time.Duration, 1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer close(tt.ch)

			spinner := &Spinner{
				mu:                &sync.Mutex{},
				frequency:         0,
				frequencyUpdateCh: tt.ch,
			}

			tmr := time.NewTimer(2 * time.Second)
			fnch := make(chan struct{})

			var err error

			go func() {
				defer close(fnch)
				err = spinner.Frequency(tt.input)
			}()

			select {
			case <-tmr.C:
				t.Fatal("function blocked")
			case <-fnch:
				tmr.Stop()
			}

			if cont := testErrCheck(t, "spinner.Frequency()", tt.err, err); !cont {
				return
			}

			if cap(tt.ch) == 1 {
				select {
				case got, ok := <-tt.ch:
					if !ok {
						t.Fatal("channel closed")
					}
					if got != tt.input {
						t.Errorf("channel receive got = %s, want %s", got, tt.input)
					}
				default:
					t.Fatal("notification channel had no messages")
				}
			}

			got := spinner.frequency
			if got != tt.input {
				t.Errorf("got = %s, want %s", got, tt.input)
			}
		})
	}
}

func TestSpinner_CharSet(t *testing.T) {
	tests := []struct {
		name         string
		stopChar     *character
		stopFailChar *character
		charSet      []string
		maxWidth     int
		err          string
	}{
		{
			name: "no_charset",
			err:  "failed to set character set:  must provide at least one string",
		},
		{
			name:     "charset",
			charSet:  CharSets[59],
			maxWidth: 3,
		},
		{
			name: "charset_with_big_stopChar",
			stopChar: &character{
				Value: "xxxx",
				Size:  4,
			},
			charSet:  CharSets[59],
			maxWidth: 4,
		},
		{
			name: "charset_with_big_stopFailChar",
			stopFailChar: &character{
				Value: "xxxxx",
				Size:  5,
			},
			charSet:  CharSets[59],
			maxWidth: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spinner := &Spinner{
				mu: &sync.Mutex{},
			}

			if tt.stopChar != nil {
				spinner.stopChar = *tt.stopChar
			}

			if tt.stopFailChar != nil {
				spinner.stopFailChar = *tt.stopFailChar
			}

			err := spinner.CharSet(tt.charSet)

			if cont := testErrCheck(t, "spinner.CharSet()", tt.err, err); !cont {
				return
			}

			charSet := make([]character, len(tt.charSet))

			for i, char := range tt.charSet {
				charSet[i] = character{
					Value: char,
					Size:  runewidth.StringWidth(char),
				}
			}

			if diff := cmp.Diff(charSet, spinner.chars); diff != "" {
				t.Fatalf("spinner.chars differs: (-want +got)\n%s", diff)
			}

			if spinner.maxWidth != tt.maxWidth {
				t.Errorf("spinner.maxWidth = %d, want %d", spinner.maxWidth, tt.maxWidth)
			}
		})
	}
}

func TestSpinner_StopCharacter(t *testing.T) {
	tests := []struct {
		name     string
		char     string
		charSize int
		mw       int
	}{
		{
			name:     "smaller_size",
			char:     "x",
			charSize: 1,
			mw:       2,
		},
		{
			name:     "larger_size",
			char:     "xxx",
			charSize: 3,
			mw:       3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spinner := &Spinner{
				mu:       &sync.Mutex{},
				maxWidth: 2,
			}

			spinner.StopCharacter(tt.char)

			c := spinner.stopChar

			if c.Value != tt.char {
				t.Fatalf("c.Value = %q, want %q", c.Value, tt.char)
			}

			if c.Size != tt.charSize {
				t.Fatalf("c.Size = %d, want %d", c.Size, tt.charSize)
			}

			if spinner.maxWidth != tt.mw {
				t.Fatalf("spinner.maxWidth = %d, want %d", spinner.maxWidth, tt.mw)
			}
		})
	}
}

func TestSpinner_StopFailCharacter(t *testing.T) {
	tests := []struct {
		name     string
		char     string
		charSize int
		mw       int
	}{
		{
			name:     "smaller_size",
			char:     "x",
			charSize: 1,
			mw:       2,
		},
		{
			name:     "larger_size",
			char:     "xxx",
			charSize: 3,
			mw:       3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spinner := &Spinner{
				mu:       &sync.Mutex{},
				maxWidth: 2,
			}

			spinner.StopFailCharacter(tt.char)

			c := spinner.stopFailChar

			if c.Value != tt.char {
				t.Fatalf("c.Value = %q, want %q", c.Value, tt.char)
			}

			if c.Size != tt.charSize {
				t.Fatalf("c.Size = %d, want %d", c.Size, tt.charSize)
			}

			if spinner.maxWidth != tt.mw {
				t.Fatalf("spinner.maxWidth = %d, want %d", spinner.maxWidth, tt.mw)
			}
		})
	}
}

func TestSpinner_Reverse(t *testing.T) {
	cfg := Config{
		Frequency: 100 * time.Millisecond,
		CharSet:   CharSets[26],
	}

	spinner, err := New(cfg)

	testErrCheck(t, "New()", "", err)

	spinner.index = 1

	csRev := make([]character, len(spinner.chars))
	copy(csRev, spinner.chars)

	for i := len(csRev)/2 - 1; i >= 0; i-- {
		opp := len(csRev) - 1 - i
		csRev[i], csRev[opp] = csRev[opp], csRev[i]
	}

	spinner.Reverse()

	if diff := cmp.Diff(csRev, spinner.chars); diff != "" {
		t.Errorf("spinner.chars differs: (-want +got)\n%s", diff)
	}

	if spinner.index != 0 {
		t.Error("index was not reset")
	}
}

func TestSpinner_erase(t *testing.T) {
	const want = "\r\033[K\r"

	buf := &bytes.Buffer{}

	testErrCheck(t, "spinner.erase()", "", erase(buf))

	got := buf.String()

	if got != want {
		t.Errorf("got = %q, want %q", got, want)
	}
}

func TestSpinner_hideCursor(t *testing.T) {
	const want = "\r\033[?25l\r"

	buf := &bytes.Buffer{}

	testErrCheck(t, "spinner.hideCursor()", "", hideCursor(buf))

	got := buf.String()

	if got != want {
		t.Errorf("got = %q, want %q", got, want)
	}
}

func TestSpinner_unhideCursor(t *testing.T) {
	const want = "\r\033[?25h\r"

	buf := &bytes.Buffer{}

	testErrCheck(t, "spinner.unhideCursor()", "", unhideCursor(buf))

	got := buf.String()

	if got != want {
		t.Errorf("got = %q, want %q", got, want)
	}
}

func TestSpinner_Start(t *testing.T) {
	tests := []struct {
		name    string
		spinner *Spinner

		err string
	}{
		{
			name: "running_spinner",
			spinner: &Spinner{
				status:          uint32Ptr(statusRunning),
				mu:              &sync.Mutex{},
				frequency:       time.Millisecond,
				colorFn:         fmt.Sprintf,
				stopColorFn:     fmt.Sprintf,
				stopFailColorFn: fmt.Sprintf,
			},
			err: "spinner already running or shutting down",
		},
		{
			name: "spinner",
			spinner: &Spinner{
				buffer:          &bytes.Buffer{},
				status:          uint32Ptr(statusStopped),
				mu:              &sync.Mutex{},
				frequency:       time.Millisecond,
				colorFn:         fmt.Sprintf,
				stopColorFn:     fmt.Sprintf,
				stopFailColorFn: fmt.Sprintf,
				stopMsg:         "stop msg",
				stopFailMsg:     "stop fail msg",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			tt.spinner.writer = buf
			_ = tt.spinner.CharSet(CharSets[26])

			err := tt.spinner.Start()

			if cont := testErrCheck(t, "Start()", tt.err, err); !cont {
				return
			}

			if tt.spinner.cancelCh == nil {
				t.Fatal("tt.spinner.cancelCh == nil")
			}

			if tt.spinner.doneCh == nil {
				t.Fatal("tt.spinner.doneCh == nil")
			}

			close(tt.spinner.cancelCh)

			<-tt.spinner.doneCh

			if buf.Len() == 0 {
				t.Fatal("painter did not write data")
			}
		})
	}
}

func TestSpinner_Pause(t *testing.T) {
	tests := []struct {
		name    string
		spinner *Spinner
		err     string
	}{
		{
			name: "not_running",
			spinner: &Spinner{
				status:  uint32Ptr(statusStopped),
				pauseCh: make(chan struct{}, 1),
			},
			err: "spinner not running",
		},
		{
			name: "running",
			spinner: &Spinner{
				status:  uint32Ptr(statusRunning),
				pauseCh: make(chan struct{}, 1),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if cont := testErrCheck(t, "Pause()", tt.err, tt.spinner.Pause()); !cont {
				return
			}

			if tt.spinner.unpauseCh == nil {
				t.Fatal("unpauseCh = nil")
			}

			if tt.spinner.unpausedCh == nil {
				t.Fatal("unpausedCh = nil")
			}

			select {
			case _, ok := <-tt.spinner.pauseCh:
				if !ok {
					t.Fatal("unexpected closed channel")
				}
			default:
				t.Fatal("expected message from pauseCh")
			}

			if s := atomic.LoadUint32(tt.spinner.status); s != statusPaused {
				t.Fatalf("status = %d, want %d", s, statusPaused)
			}
		})
	}
}

func TestSpinner_Unpause(t *testing.T) {
	tests := []struct {
		name    string
		spinner *Spinner
		err     string
	}{
		{
			name: "not_paused",
			spinner: &Spinner{
				status:     uint32Ptr(statusStopped),
				unpauseCh:  make(chan struct{}),
				unpausedCh: make(chan struct{}),
			},
			err: "spinner not paused",
		},
		{
			name: "running",
			spinner: &Spinner{
				status:     uint32Ptr(statusPaused),
				unpauseCh:  make(chan struct{}),
				unpausedCh: make(chan struct{}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := tt.spinner.unpauseCh
			close(tt.spinner.unpausedCh)

			if cont := testErrCheck(t, "Unpause()", tt.err, tt.spinner.Unpause()); !cont {
				return
			}

			if tt.spinner.unpauseCh != nil {
				t.Fatal("unpauseCh != nil")
			}

			if tt.spinner.unpausedCh != nil {
				t.Fatal("unpausedCh != nil")
			}

			select {
			case _, ok := <-ch:
				if ok {
					t.Fatal("unexpected open channel")
				}
			default:
				t.Fatal("expected unpauseCh closed")
			}

			if s := atomic.LoadUint32(tt.spinner.status); s != statusRunning {
				t.Fatalf("status = %d, want %d", s, statusRunning)
			}
		})
	}
}

func TestSpinner_Stop(t *testing.T) {
	tests := []struct {
		name    string
		spinner *Spinner
		err     string
	}{
		{
			name: "not_running",
			spinner: &Spinner{
				mu:       &sync.Mutex{},
				status:   uint32Ptr(statusStopped),
				cancelCh: make(chan struct{}),
				doneCh:   make(chan struct{}),
			},
			err: "spinner not running or paused",
		},
		{
			name: "running",
			spinner: &Spinner{
				mu:       &sync.Mutex{},
				status:   uint32Ptr(statusRunning),
				cancelCh: make(chan struct{}),
				doneCh:   make(chan struct{}),
			},
		},
		{
			name: "paused",
			spinner: &Spinner{
				mu:         &sync.Mutex{},
				status:     uint32Ptr(statusPaused),
				cancelCh:   make(chan struct{}),
				doneCh:     make(chan struct{}),
				unpauseCh:  make(chan struct{}),
				unpausedCh: make(chan struct{}),
			},
		},
	}

	for _, tt := range tests {
		tt := tt // create local copy
		t.Run(tt.name, func(t *testing.T) {
			if tt.spinner.unpausedCh != nil {
				close(tt.spinner.unpausedCh)
			}

			var ok bool
			wait := make(chan struct{})

			go func(doneCh, cancelCh chan struct{}) {
				close(doneCh)
				_, ok = <-cancelCh
				close(wait)
			}(tt.spinner.doneCh, tt.spinner.cancelCh)

			if cont := testErrCheck(t, "spinner.Stop()", tt.err, tt.spinner.Stop()); !cont {
				return
			}

			<-wait

			if !ok {
				t.Error("expected stop() to send message and not close channel")
			}

			if tt.spinner.index != 0 {
				t.Errorf("tt.spinner.index = %d, want 0", tt.spinner.index)
			}

			if tt.spinner.cancelCh != nil {
				t.Error("tt.spinner.cancelCh is not nil")
			}

			if tt.spinner.doneCh != nil {
				t.Error("tt.spinner.doneCh is not nil")
			}

			status := atomic.LoadUint32(tt.spinner.status)
			if status != 0 {
				t.Errorf("tt.spinner.status = %d, want 0", status)
			}
		})
	}
}

func TestSpinner_StopFail(t *testing.T) {
	tests := []struct {
		name    string
		spinner *Spinner
		err     string
	}{
		{
			name: "not_running",
			spinner: &Spinner{
				mu:       &sync.Mutex{},
				status:   uint32Ptr(statusStopped),
				cancelCh: make(chan struct{}),
				doneCh:   make(chan struct{}),
			},
			err: "spinner not running or paused",
		},
		{
			name: "running",
			spinner: &Spinner{
				mu:       &sync.Mutex{},
				status:   uint32Ptr(statusRunning),
				cancelCh: make(chan struct{}),
				doneCh:   make(chan struct{}),
			},
		},
	}

	for _, tt := range tests {
		tt := tt // create local copy
		t.Run(tt.name, func(t *testing.T) {
			var ok bool
			wait := make(chan struct{})

			go func(doneCh, cancelCh chan struct{}) {
				close(doneCh)
				_, ok = <-cancelCh
				close(wait)
			}(tt.spinner.doneCh, tt.spinner.cancelCh)

			if cont := testErrCheck(t, "spinner.Stop()", tt.err, tt.spinner.StopFail()); !cont {
				return
			}

			<-wait

			if ok {
				t.Error("expected stop() to not send message and instead close the channel")
			}

			if tt.spinner.index != 0 {
				t.Errorf("tt.spinner.index = %d, want 0", tt.spinner.index)
			}

			if tt.spinner.cancelCh != nil {
				t.Error("tt.spinner.cancelCh is not nil")
			}

			if tt.spinner.doneCh != nil {
				t.Error("tt.spinner.doneCh is not nil")
			}

			status := atomic.LoadUint32(tt.spinner.status)
			if status != 0 {
				t.Errorf("tt.spinner.status = %d, want 0", status)
			}
		})
	}
}

func TestSpinner_paintUpdate(t *testing.T) {
	tests := []struct {
		name    string
		spinner *Spinner
		want    string
	}{
		{
			name: "spinner_no_hide_cursor",
			spinner: &Spinner{
				buffer:    &bytes.Buffer{},
				mu:        &sync.Mutex{},
				prefix:    "a",
				message:   "msg",
				suffix:    " ",
				maxWidth:  1,
				colorFn:   fmt.Sprintf,
				chars:     []character{{Value: "y", Size: 1}, {Value: "z", Size: 1}},
				frequency: 10,
			},
			want: "\r\033[K\ray msg\r\033[K\raz msg\r\033[K\raz msg\r\033[K\ray msg",
		},
		{
			name: "spinner_no_hide_cursor_auto_cursor",
			spinner: &Spinner{
				buffer:          &bytes.Buffer{},
				mu:              &sync.Mutex{},
				prefix:          "a",
				message:         "msg",
				suffix:          " ",
				maxWidth:        1,
				colorFn:         fmt.Sprintf,
				chars:           []character{{Value: "y", Size: 1}, {Value: "z", Size: 1}},
				frequency:       10,
				suffixAutoColon: true,
			},
			want: "\r\033[K\ray : msg\r\033[K\raz : msg\r\033[K\raz : msg\r\033[K\ray : msg",
		},
		{
			name: "spinner_hide_cursor",
			spinner: &Spinner{
				buffer:       &bytes.Buffer{},
				cursorHidden: true,
				mu:           &sync.Mutex{},
				prefix:       "a",
				message:      "msg",
				suffix:       " ",
				maxWidth:     1,
				colorFn:      fmt.Sprintf,
				chars:        []character{{Value: "y", Size: 1}, {Value: "z", Size: 1}},
				frequency:    10,
			},
			want: "\r\033[K\r\r\033[?25l\ray msg\r\033[K\r\r\033[?25l\raz msg\r\033[K\r\r\033[?25l\raz msg\r\033[K\r\r\033[?25l\ray msg",
		},
		{
			name: "spinner_hide_cursor_dumbterm",
			spinner: &Spinner{
				buffer:       &bytes.Buffer{},
				cursorHidden: true,
				mu:           &sync.Mutex{},
				prefix:       "a",
				message:      "msg",
				suffix:       " ",
				maxWidth:     1,
				colorFn:      fmt.Sprintf,
				chars:        []character{{Value: "y", Size: 1}, {Value: "z", Size: 1}},
				frequency:    10,
				isDumbTerm:   true,
			},
			want: "\r\ray msg\r      \raz msg\r      \raz msg\r      \ray msg",
		},
		{
			name: "spinner_empty_print",
			spinner: &Spinner{
				buffer:    &bytes.Buffer{},
				mu:        &sync.Mutex{},
				maxWidth:  0,
				colorFn:   fmt.Sprintf,
				chars:     []character{{Value: "", Size: 0}},
				frequency: 10,
			},
			want: "\r\033[K\r\r\033[K\r\r\033[K\r\r\033[K\r",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			tt.spinner.writer = buf

			tm := time.NewTimer(10 * time.Millisecond)

			tt.spinner.paintUpdate(tm, false)
			tt.spinner.paintUpdate(tm, false)
			tt.spinner.paintUpdate(tm, true)
			tt.spinner.paintUpdate(tm, false)
			tm.Stop()

			got := buf.String()

			if got != tt.want {
				t.Errorf("got = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSpinner_paintStop(t *testing.T) {
	tests := []struct {
		name    string
		ok      bool
		spinner *Spinner
		want    string
	}{
		{
			name: "ok",
			ok:   true,
			spinner: &Spinner{
				buffer:      &bytes.Buffer{},
				mu:          &sync.Mutex{},
				prefix:      "a",
				suffix:      " ",
				maxWidth:    1,
				stopColorFn: fmt.Sprintf,
				stopChar:    character{Value: "x", Size: 1},
				stopMsg:     "stop",
			},
			want: "\r\033[K\rax stop\n",
		},
		{
			name: "ok_auto_colon",
			ok:   true,
			spinner: &Spinner{
				buffer:          &bytes.Buffer{},
				mu:              &sync.Mutex{},
				prefix:          "a",
				suffix:          " ",
				maxWidth:        1,
				stopColorFn:     fmt.Sprintf,
				stopChar:        character{Value: "x", Size: 1},
				stopMsg:         "stop",
				suffixAutoColon: true,
			},
			want: "\r\033[K\rax : stop\n",
		},
		{
			name: "ok_auto_colon_no_msg",
			ok:   true,
			spinner: &Spinner{
				buffer:          &bytes.Buffer{},
				mu:              &sync.Mutex{},
				prefix:          "a",
				suffix:          " ",
				maxWidth:        1,
				stopColorFn:     fmt.Sprintf,
				stopChar:        character{Value: "x", Size: 1},
				stopMsg:         "",
				suffixAutoColon: true,
			},
			want: "\r\033[K\rax \n",
		},
		{
			name: "ok_unhide",
			ok:   true,
			spinner: &Spinner{
				buffer:       &bytes.Buffer{},
				mu:           &sync.Mutex{},
				cursorHidden: true,
				prefix:       "a",
				suffix:       " ",
				maxWidth:     1,
				stopColorFn:  fmt.Sprintf,
				stopChar:     character{Value: "x", Size: 1},
				stopMsg:      "stop",
			},
			want: "\r\033[K\r\r\033[?25h\rax stop\n",
		},
		{
			name: "ok_unhide_dumbterm",
			ok:   true,
			spinner: &Spinner{
				buffer:       &bytes.Buffer{},
				mu:           &sync.Mutex{},
				cursorHidden: true,
				prefix:       "a",
				suffix:       " ",
				maxWidth:     1,
				stopColorFn:  fmt.Sprintf,
				stopChar:     character{Value: "x", Size: 1},
				stopMsg:      "stop",
				isDumbTerm:   true,
				lastPrintLen: 10,
			},
			want: "\r          \rax stop\n",
		},
		{
			name: "fail",
			spinner: &Spinner{
				buffer:          &bytes.Buffer{},
				mu:              &sync.Mutex{},
				prefix:          "a",
				suffix:          " ",
				maxWidth:        1,
				stopFailColorFn: fmt.Sprintf,
				stopFailChar:    character{Value: "y", Size: 1},
				stopFailMsg:     "stop",
			},
			want: "\r\033[K\ray stop\n",
		},
		{
			name: "fail_no_char_no_msg",
			spinner: &Spinner{
				buffer:          &bytes.Buffer{},
				mu:              &sync.Mutex{},
				prefix:          "a",
				suffix:          " ",
				maxWidth:        1,
				stopFailColorFn: fmt.Sprintf,
			},
			want: "\r\033[K\r",
		},
		{
			name: "fail_no_char_no_msg_dumb_term",
			spinner: &Spinner{
				buffer:          &bytes.Buffer{},
				mu:              &sync.Mutex{},
				prefix:          "a",
				suffix:          " ",
				maxWidth:        1,
				isDumbTerm:      true,
				stopFailColorFn: fmt.Sprintf,
			},
			want: "\r\r",
		},
		{
			name: "fail_colorall",
			spinner: &Spinner{
				buffer:   &bytes.Buffer{},
				mu:       &sync.Mutex{},
				prefix:   "a",
				suffix:   " ",
				maxWidth: 1,
				stopFailColorFn: func(format string, a ...interface{}) string {
					return fmt.Sprintf("fullColor: %s", fmt.Sprintf(format, a...))
				},
				stopFailChar: character{Value: "y", Size: 1},
				stopFailMsg:  "stop",
				colorAll:     true,
			},
			want: "\r\033[K\rfullColor: ay stop\n",
		},
		{
			name: "fail_colorall_no_char",
			spinner: &Spinner{
				buffer:   &bytes.Buffer{},
				mu:       &sync.Mutex{},
				prefix:   "a",
				suffix:   " ",
				maxWidth: 0,
				stopFailColorFn: func(format string, a ...interface{}) string {
					return fmt.Sprintf("fullColor: %s", fmt.Sprintf(format, a...))
				},
				stopFailChar: character{Value: "", Size: 0},
				stopFailMsg:  "stop",
				colorAll:     true,
			},
			want: "\r\033[K\rfullColor: stop\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			tt.spinner.writer = buf

			tt.spinner.paintStop(tt.ok)

			got := buf.String()

			if got != tt.want {
				t.Errorf("got = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func Test_handleFrequencyUpdate(t *testing.T) {
	tests := []struct {
		name         string
		newFrequency time.Duration
		lastTickAgo  time.Duration
		shouldTick   time.Duration
	}{
		{
			name:         "moreTime",
			newFrequency: 200 * time.Millisecond,
			lastTickAgo:  100 * time.Millisecond,
			shouldTick:   (100 * time.Millisecond) + (500 * time.Microsecond),
		},
		{
			name:         "lessTime",
			newFrequency: 100 * time.Millisecond,
			lastTickAgo:  200 * time.Millisecond,
			shouldTick:   100 * time.Microsecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timer := time.NewTimer(0)
			lastTick := time.Now().Add(-tt.lastTickAgo)

			time.Sleep(10 * time.Microsecond)

			handleFrequencyUpdate(tt.newFrequency, timer, lastTick)

			testTimer := time.NewTimer(tt.shouldTick)

			select {
			case <-timer.C:
				testTimer.Stop()
			case <-testTimer.C:
				timer.Stop()
				t.Fatal("timer didn't fire when expected")
			}
		})
	}
}

func Test_setToCharSlice(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		wantNil   bool
		wantChars []character
		wantSize  int
	}{
		{
			name:    "nil",
			wantNil: true,
		},
		{
			name:      "full",
			input:     []string{"x", "zzz"},
			wantChars: []character{{Value: "x", Size: 1}, {Value: "zzz", Size: 3}},
			wantSize:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chars, size := setToCharSlice(tt.input)

			if size != tt.wantSize {
				t.Errorf("size = %d, want %d", size, tt.wantSize)
			}

			if tt.wantNil && chars != nil {
				t.Fatal("chars not nil")
			}

			for i := range chars {
				if x, y := chars[i], tt.wantChars[i]; x != y {
					t.Errorf("chars[%d] = %#v, want %#v", i, x, y)
				}
			}
		})
	}
}

func TestSpinner_painter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	const want = "\r\033[K\ray msg\r\033[K\ray othermsg\r\033[K\raz msg\r\033[K\ray msg\r\x1b[K\rav stop\n"

	buf := &bytes.Buffer{}

	cancel, done, dataUpdate, pause := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	frequencyUpdate := make(chan time.Duration, 1)

	spinner := &Spinner{
		buffer:            &bytes.Buffer{},
		mu:                &sync.Mutex{},
		writer:            buf,
		prefix:            "a",
		message:           "msg",
		suffix:            " ",
		maxWidth:          1,
		colorFn:           fmt.Sprintf,
		chars:             []character{{Value: "y", Size: 1}, {Value: "z", Size: 1}},
		stopColorFn:       fmt.Sprintf,
		stopMsg:           "stop",
		stopChar:          character{Value: "v", Size: 1},
		frequency:         2000 * time.Millisecond,
		cancelCh:          cancel,
		doneCh:            done,
		dataUpdateCh:      dataUpdate,
		frequencyUpdateCh: frequencyUpdate,
	}

	go spinner.painter(cancel, dataUpdate, pause, done, frequencyUpdate)

	time.Sleep(500 * time.Millisecond)

	spinner.mu.Lock()

	spinner.message = "othermsg"
	spinner.dataUpdateCh <- struct{}{}

	spinner.mu.Unlock()

	time.Sleep(500 * time.Millisecond)

	spinner.unpauseCh, spinner.unpausedCh = make(chan struct{}), make(chan struct{})
	pause <- struct{}{}

	close(spinner.unpauseCh)
	_, ok := <-spinner.unpausedCh

	if ok {
		t.Fatal("unexpected successful channel receive")
	}

	spinner.unpauseCh = nil
	spinner.unpausedCh = nil

	spinner.mu.Lock()

	spinner.message = "msg"
	spinner.frequency = 1000 * time.Millisecond
	frequencyUpdate <- 1000 * time.Millisecond

	spinner.mu.Unlock()

	time.Sleep(1200 * time.Millisecond)

	cancel <- struct{}{}

	<-done

	got := buf.String()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("output differs: (-want / +got)\n%s", diff)
	}
}

func TestSpinnerStatus_String(t *testing.T) {
	tests := []struct {
		name string
		ss   SpinnerStatus
		want string
	}{
		{
			name: "stopped",
			ss:   SpinnerStopped,
			want: "stopped",
		},
		{
			name: "starting",
			ss:   SpinnerStarting,
			want: "starting",
		},
		{
			name: "running",
			ss:   SpinnerRunning,
			want: "running",
		},
		{
			name: "stopping",
			ss:   SpinnerStopping,
			want: "stopping",
		},
		{
			name: "pausing",
			ss:   SpinnerPausing,
			want: "pausing",
		},
		{
			name: "paused",
			ss:   SpinnerPaused,
			want: "paused",
		},
		{
			name: "unpausing",
			ss:   SpinnerUnpausing,
			want: "unpausing",
		},
		{
			name: "unknown",
			ss:   42,
			want: "unknown (42)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ss.String(); got != tt.want {
				t.Errorf("got = %#v, got %#v", got, tt.want)
			}
		})
	}
}
