package yacspin

import (
	"io"
	"os"
	"strings"
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
			err:    "cfg.Delay must be greater than 0",
		},
		{
			name:     "config_with_delay_and_default_writer",
			maxWidth: 1,
			writer:   os.Stdout,
			cfg: Config{
				Delay: 100 * time.Millisecond,
			},
		},
		{
			name:   "config_with_delay_and_invalid_colors",
			writer: os.Stdout,
			cfg: Config{
				Delay:  100 * time.Millisecond,
				Colors: []string{"invalid"},
			},
			err: "failed to build color function: invalid is not a valid color",
		},
		{
			name:   "config_with_delay_and_invalid_stopColors",
			writer: os.Stdout,
			cfg: Config{
				Delay:      100 * time.Millisecond,
				StopColors: []string{"invalid"},
			},
			err: "failed to build stop color function: invalid is not a valid color",
		},
		{
			name:   "config_with_delay_and_invalid_stopFailColors",
			writer: os.Stdout,
			cfg: Config{
				Delay:          100 * time.Millisecond,
				StopFailColors: []string{"invalid"},
			},
			err: "failed to build stop fail color function: invalid is not a valid color",
		},
		{
			name:     "full_config",
			writer:   os.Stderr,
			maxWidth: 3,
			cfg: Config{
				Delay:             100 * time.Millisecond,
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

			if spinner.active == nil {
				t.Fatal("spinner.active is nil")
			}

			if spinner.duCh == nil {
				t.Fatal("spinner.duCh is nil")
			}

			if spinner.delayDuration == nil {
				t.Fatal("spinner.delayDuration is nil")
			}

			dd := atomicDuration(spinner.delayDuration)
			if dd != tt.cfg.Delay {
				t.Errorf("spinner.delayDuration = %s, want %s", dd, tt.cfg.Delay)
			}

			if spinner.writer == nil {
				t.Fatal("spinner.writer is nil")
			}

			if spinner.writer != tt.writer {
				t.Errorf("spinner.writer = %#v, want %#v", spinner.writer, tt.writer)
			}

			if spinner.prefix == nil {
				t.Fatal("spinner.prefix is nil")
			}

			prefix := atomicString(spinner.prefix)
			if prefix != tt.cfg.Prefix {
				t.Errorf("spinner.prefix = %q, want %q", prefix, tt.cfg.Prefix)
			}

			if spinner.suffix == nil {
				t.Fatal("spinner.suffix is nil")
			}

			suffix := atomicString(spinner.suffix)
			if suffix != tt.cfg.Suffix {
				t.Errorf("spinner.suffix = %q, want %q", suffix, tt.cfg.Suffix)
			}

			if spinner.message == nil {
				t.Fatal("spinner.message is nil")
			}

			message := atomicString(spinner.message)
			if message != tt.cfg.Message {
				t.Errorf("spinner.message = %q, want %q", message, tt.cfg.Message)
			}

			if spinner.stopMsg == nil {
				t.Fatal("spinner.stopMsg is nil")
			}

			stopMsg := atomicString(spinner.stopMsg)
			if stopMsg != tt.cfg.StopMessage {
				t.Errorf("spinner.stopMsg = %q, want %q", stopMsg, tt.cfg.StopMessage)
			}

			if spinner.stopChar == nil {
				t.Fatal("spinner.stopChar is nil")
			}

			stopChar := atomicCharacter(spinner.stopChar)
			sc := character{Value: tt.cfg.StopCharacter, Size: runewidth.StringWidth(tt.cfg.StopCharacter)}
			if stopChar != sc {
				t.Errorf("spinner.stopChar = %#v, want %#v", stopChar, sc)
			}

			if spinner.stopFailMsg == nil {
				t.Fatal("spinner.stopFailMsg is nil")
			}

			stopFailMsg := atomicString(spinner.stopFailMsg)
			if stopFailMsg != tt.cfg.StopFailMessage {
				t.Errorf("spinner.stopFailMsg = %q, want %q", stopFailMsg, tt.cfg.StopFailMessage)
			}

			if spinner.stopFailChar == nil {
				t.Fatal("spinner.stopFailChar is nil")
			}

			stopFailChar := atomicCharacter(spinner.stopFailChar)
			sfc := character{Value: tt.cfg.StopFailCharacter, Size: runewidth.StringWidth(tt.cfg.StopFailCharacter)}
			if stopFailChar != sfc {
				t.Errorf("spinner.stopFailChar = %#v, want %#v", stopFailChar, sfc)
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
			sfn := atomicColorFn(spinner.colorFn)

			gotStr, wantStr := sfn("%s: %d", "test string", 42), tfn("%s: %d", "test string", 42)

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
			sfn = atomicColorFn(spinner.stopColorFn)

			gotStr, wantStr = sfn("%s: %d", "test string", 42), tfn("%s: %d", "test string", 42)

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
			sfn = atomicColorFn(spinner.stopFailColorFn)

			gotStr, wantStr = sfn("%s: %d", "test string", 42), tfn("%s: %d", "test string", 42)

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
