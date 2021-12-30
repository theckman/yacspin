# demo

This program is the helper used to create the gifs in the README. What follows
are notes for when I do this in the future.

### Overview

The software in this `demo` folder is used to demonstrate all of the spinners
that exist, and to help generate the GIFs used for the README.md file in the
root of this repo.

There are two helper scripts in the `scripts/` folder. `ffmpeg.sh` helps with
converting the h.264 .mp4 files into .gif files. The `rename.go` program then
helps rename those .gif files into the right name for committing to the
repository.

The GIFs are stored in a sister GitHub repository, [github.com/theckman/yacspin-gifs](https://github.com/theckman/yacspin-gifs)
to avoid cluttering up this repo with binary files (images).

#### Checklist

- [ ] start a screen recording of the terminal using Apple QuickTime Player
- [ ] start the demo program (`main.go`) and let it run entirely before stopping the QuickTime recording
- [ ] load the .mov file into Adobe Preimere, and cut the clip down into the individual 10 second clips of each spinner
- [ ] turn the clips in the original Sequence into Subsequences
- [ ] use media encoder to export those subsequences to their own files, matching source with Adaptive High Bitrate
- [ ] use the `scripts/ffmpeg.sh` script to convert the `.mp4` files into `.gif` files
- [ ] `go run` the `scripts/rename.go` program, to rename the `.gif` files to match the names we expect

#### QuickTime Capture Sizes

When using the Apple QuickTime Player to capture the screen recording, you can
have it only capture a specific area of the screen. These are the capture sizes
I played with on my Apple 16" M1 Max MBP laptop.

Please note, that the video files rendered from these captures tend to have
about 2x the resolution. Keep that in mind when deciding how large of an area
you want to capture.

Capture Size (pixels) | Font Size
----------------------|----------
650w x 24h | 18
340w x 12h | 10
1300w x 47h | 38
