package yacspin

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fatih/color"
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

func Test_validColor(t *testing.T) {
	validColors := []string{
		"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
		"reset", "bold", "faint", "italic", "underline", "blinkslow", "blinkrapid", "reversevideo", "concealed", "crossedout",
		"fgBlack", "fgRed", "fgGreen", "fgYellow", "fgBlue", "fgMagenta", "fgCyan", "fgWhite",
		"fgHiBlack", "fgHiRed", "fgHiGreen", "fgHiYellow", "fgHiBlue", "fgHiMagenta", "fgHiCyan", "fgHiWhite",
		"bgBlack", "bgRed", "bgGreen", "bgYellow", "bgBlue", "bgMagenta", "bgCyan", "bgWhite",
		"bgHiBlack", "bgHiRed", "bgHiGreen", "bgHiYellow", "bgHiBlue", "bgHiMagenta", "bgHiCyan", "bgHiWhite",
	}

	tests := []struct {
		name  string
		color string
		want  bool
	}{
		{
			name:  "invalid",
			color: "invalid",
			want:  false,
		},
	}

	for _, c := range validColors {
		tests = append(tests, struct {
			name  string
			color string
			want  bool
		}{
			name:  c,
			color: c,
			want:  true,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validColor(tt.color); got != tt.want {
				t.Fatalf("validColor(%q) = %t, want %t", tt.color, got, tt.want)
			}
		})
	}
}

func Test_colorFunc(t *testing.T) {
	tests := []struct {
		name   string
		colors []string
		err    string
	}{
		{
			name: "no_color",
		},
		{
			name:   "color",
			colors: []string{"fgHiGreen"},
		},
		{
			name:   "colors",
			colors: []string{"fgHiGreen", "bgRed"},
		},
		{
			name:   "invalid_color",
			colors: []string{"fgHiGreen", "invalid", "bgRed"},
			err:    "invalid is not a valid color",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tfn func(format string, a ...interface{}) string

			if len(tt.colors) == 0 {
				tfn = fmt.Sprintf
			} else {
				a := make([]color.Attribute, len(tt.colors))

				for i, c := range tt.colors {
					ca, ok := colorAttributeMap[c]
					if !ok {
						continue
					}
					a[i] = ca
				}

				tfn = color.New(a...).SprintfFunc()
			}

			fn, err := colorFunc(tt.colors...)

			if cont := testErrCheck(t, "colorFunc()", tt.err, err); !cont {
				return
			}

			if fn == nil {
				t.Fatal("fn is nil")
			}

			got, want := fn("%s: %d", "test value", 42), tfn("%s: %d", "test value", 42)

			if got != want {
				t.Fatalf(`fn("%%s: %%d", "test value", 42) = %q, want %q`, got, want)
			}
		})
	}
}
