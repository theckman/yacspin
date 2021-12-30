// This program was used to generate the gifs in the README file

package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/theckman/yacspin"
)

func main() {
	// disable GC
	debug.SetGCPercent(-1)

	cfg := yacspin.Config{
		Frequency:       200 * time.Millisecond,
		CharSet:         yacspin.CharSets[36],
		SuffixAutoColon: true,
		Suffix:          " example spinner",
		Message:         "initial message",
		Colors:          []string{"fgYellow"},
	}

	spinner, err := yacspin.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create spinner: %v\n", err)
		os.Exit(1)
	}

	// handle SIGINT / SIGTERM without needing terminal reset
	sigc := make(chan os.Signal, 2)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigc
		_ = spinner.Stop()
		os.Exit(0)
	}()

	// run GC once before we start to render
	runtime.GC()

	time.Sleep(3 * time.Second)

	for i := 0; i < len(yacspin.CharSets); i++ {
		spinner.Message("initial message")

		// interesting charsets for recording sizing: 19, 36
		_ = spinner.CharSet(yacspin.CharSets[i])

		_ = spinner.Start()

		time.Sleep(5 * time.Second)

		spinner.Message("updated message")

		time.Sleep(5 * time.Second)

		_ = spinner.Stop()
	}
}
