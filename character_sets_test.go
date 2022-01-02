package yacspin

import (
	"fmt"
	"testing"
	"time"
)

func TestCharSets(t *testing.T) {
	spinner, err := New(Config{Frequency: time.Second})
	testErrCheck(t, "New()", "", err)

	for i, cs := range CharSets {
		name := fmt.Sprintf("spinner.CharSet(CharSets[%d])", i)
		err := spinner.CharSet(cs)

		testErrCheck(t, name, "", err)
	}
}
