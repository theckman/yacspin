package yacspin

import (
	"fmt"
	"testing"
	"time"
)

func TestCharSets(t *testing.T) {
	spinner, err := New(Config{Frequency: time.Second})
	testErrCheck(t, "New()", "", err)

	for i := 0; i < len(CharSets); i++ {
		name := fmt.Sprintf("spinner.CharSet(CharSets[%d])", i)
		err := spinner.CharSet(CharSets[i])
		testErrCheck(t, name, "", err)
	}
}
