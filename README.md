# Yet Another CLi Spinner (for Go)
[![License](https://img.shields.io/github/license/theckman/yacspin.svg)](https://github.com/theckman/yacspin/blob/master/LICENSE)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/theckman/yacspin)
[![Latest Git Tag](https://img.shields.io/github/tag/theckman/yacspin.svg)](https://github.com/theckman/yacspin/releases)
[![Travis master Build Status](https://img.shields.io/travis/theckman/yacspin/master.svg?label=TravisCI)](https://travis-ci.org/theckman/yacspin/branches)
[![Go Report Card](https://goreportcard.com/badge/github.com/theckman/yacspin)](https://goreportcard.com/report/github.com/theckman/yacspin)
[![Codecov](https://img.shields.io/codecov/c/github/theckman/yacspin)](https://codecov.io/gh/theckman/yacspin)

Package `yacspin` provides yet another CLi spinner for Go, taking inspiration
(and some utility code) from the https://github.com/briandowns/spinner project.
Specifically `yacspin` borrows the default character sets, and color mappings to
github.com/fatih/color colors, from that project.

## Usage

```
go get github.com/theckman/yacspin
```

Within the `yacspin` package there are some default spinners stored in the
`yacspin.CharSets` variable, and you can also provide your own. There is also a
list of known colors in the `yacspin.ValidColors` variable.

```Go
cfg := yacspin.Config{
	Delay:           100 * time.Millisecond,
	CharSet:         yacspin.CharSets[59],
	Suffix:          " backing up database to S3",
    SuffixAutoColon: true,
	Message:         "exporting data",
	StopCharacter:   "✓",
	StopColors:      []string{"fgGreen"},
}

spinner, err := yacspin.New(cfg)
// handle the error

spinner.Start()

// doing some work
time.Sleep(2 * time.Second)

spinner.Message("uploading data")

// upload...
time.Sleep(2 * time.Second)

spinner.Stop()
```
