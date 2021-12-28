package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/theckman/yacspin"
)

func main() {
	// createSpinnerFromStruct()
	useSpinner(createSpinnerFromStruct(), true)  // failure message
	useSpinner(createSpinnerFromStruct(), false) // success message

	// createSpinnerFromMethods()
	useSpinner(createSpinnerFromMethods(), true)  // failure message
	useSpinner(createSpinnerFromMethods(), false) // success message
}

// createSpinnerFromStruct shows configuring the spinner mostly from the Config
// struct.
func createSpinnerFromStruct() *yacspin.Spinner {
	cfg := yacspin.Config{
		Frequency:         100 * time.Millisecond,
		Colors:            []string{"fgYellow"},
		CharSet:           yacspin.CharSets[11],
		Suffix:            " ",
		SuffixAutoColon:   true,
		Message:           "only spinner is colored",
		StopCharacter:     "✓",
		StopColors:        []string{"fgGreen"},
		StopMessage:       "done",
		StopFailCharacter: "✗",
		StopFailColors:    []string{"fgRed"},
		StopFailMessage:   "failed",
	}

	s, err := yacspin.New(cfg)
	if err != nil {
		exitf("failed to make spinner from struct: %v", err)
	}

	return s
}

// createSpinnerFromMethods shows configuring the spinner mostly from its
// methods.
func createSpinnerFromMethods() *yacspin.Spinner {
	cfg := yacspin.Config{
		Frequency:       100 * time.Millisecond,
		ColorAll:        true,
		SuffixAutoColon: true,
		Message:         "spinner and text is colored",
	}

	s, err := yacspin.New(cfg)
	if err != nil {
		exitf("failed to generate spinner from methods: %v", err)
	}

	if err := s.CharSet(yacspin.CharSets[11]); err != nil {
		exitf("failed to set charset: %v", err)
	}

	if err := s.Colors("fgYellow"); err != nil {
		exitf("failed to set color: %v", err)
	}

	if err := s.StopColors("fgGreen"); err != nil {
		exitf("failed to set stop colors: %v", err)
	}

	if err := s.StopFailColors("fgRed"); err != nil {
		exitf("failed to set stop fail colors: %v", err)
	}

	s.Suffix(" ")
	s.StopCharacter("✓")
	s.StopMessage("done")
	s.StopFailCharacter("✗")
	s.StopFailMessage("failed")

	return s
}

// useSpinner utilizes the differently configured spinners, if shouldFail is
// true it uses the .StopFail method instead of .Stop.
func useSpinner(spinner *yacspin.Spinner, shouldFail bool) {
	// handle spinner cleanup on interrupts
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	defer signal.Stop(sigCh)

	go func() { // this is just an example signal handler, should be more robust
		<-sigCh

		spinner.StopFailMessage("interrupted")

		// ignoring error intentionally
		_ = spinner.StopFail()

		os.Exit(0)
	}()

	// start animating the spinner
	if err := spinner.Start(); err != nil {
		exitf("failed to start spinner: %v", err)
	}

	// let spinner animation render for a bit
	time.Sleep(2 * time.Second)

	// pause spinner to do an "atomic" config update
	if err := spinner.Pause(); err != nil {
		exitf("failed to pause spinner: %v", err)
	}

	spinner.Suffix(" uploading files")
	spinner.Message("")

	// start to animate the spinner again
	if err := spinner.Unpause(); err != nil {
		exitf("failed to unpause spinner: %v", err)
	}

	// let spinner animation render for a bit
	time.Sleep(time.Second)

	// simulate uploading a series of different files
	for _, f := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		spinner.Message(fmt.Sprintf("%s.zip", f))

		time.Sleep(150 * time.Millisecond)
	}

	// let spinner animation render for a bit
	time.Sleep(1500 * time.Millisecond)

	if shouldFail {
		if err := spinner.StopFail(); err != nil {
			exitf("failed to stopfail: %v", err)
		}
	} else {
		if err := spinner.Stop(); err != nil {
			exitf("failed to stop: %v", err)
		}
	}
}

func exitf(format string, a ...interface{}) {
	fmt.Printf(format, a...)
	os.Exit(1)
}
