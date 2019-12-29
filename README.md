# Yet Another CLi Spinner (for Go)
[![License](https://img.shields.io/github/license/theckman/yacspin.svg)](https://github.com/theckman/yacspin/blob/master/LICENSE)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/theckman/yacspin)

Package `yacspin` provides yet another CLi spinner for Go, taking inspiration
(and some utility code) from the https://github.com/briandowns/spinner project.
Specifically `yacspin` borrows the default character sets, and color mappings to
github.com/fatih/color colors, from that project.

## Usage

Within the `yacspin` package there are some default spinners stored in the
`yacspin.CharSets` variable, but you can also provide your own. There is also a
list of known colors in the `yacspin.ValidColors` variable.

```Go
cfg := yacspin.Config{
	Delay:         100 * time.Millisecond,
	CharSet:       yacspin.CharSets[59],
	Suffix:        " backing up database to S3",
	Message:       "exporting data",
	StopCharacter: "✓",
	StopColors:    []string{"fgGreen"},
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
