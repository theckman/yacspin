package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func main() {
	var sim bool
	var glob string

	flag.BoolVar(&sim, "sim", false, "simulate")
	flag.StringVar(&glob, "glob", "", "glob to use for matching files, must end in *.gif")
	flag.Parse()

	if !strings.HasSuffix(glob, "*.gif") {
		fmt.Fprintln(os.Stderr, "-glob flag required and must end in *.gif")
		os.Exit(int(syscall.EINVAL))
	}

	m, err := filepath.Glob(glob)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to glob %s: %v\n", glob, err)
		os.Exit(1)
	}

	if len(m) == 0 {
		fmt.Fprintf(os.Stderr, "no files matched glob %s\n", glob)
		os.Exit(1)
	}

	idx := strings.Index(glob, "*")

	for _, match := range m {
		num := strings.TrimSuffix(match[idx:], ".gif")

		ni, err := strconv.Atoi(num)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to convert number in %s to integer: %v\n", match, err)
			os.Exit(1)
		}

		ni--

		newName := fmt.Sprintf("%d.gif", ni)

		if sim {
			fmt.Printf("would rename %s to %s\n", match, newName)
			continue
		}

		if err := os.Rename(match, newName); err != nil {
			fmt.Fprintf(os.Stderr, "failed to rename %s to %s\n", match, newName)
			os.Exit(1)
		}
	}
}
