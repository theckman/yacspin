#!/bin/bash

####
#
# Convert *.mp4 files in the current directory to GIFs at 10 FPS
#
####

function main() {
    for file in *.mp4;
    do
        basename=$(echo "${file}" | sed -e 's/\.mp4$//')

        ffmpeg -i "${basename}.mp4" -vf "fps=10,split[s0][s1];[s0]palettegen[p];[s1][p]paletteuse" -loop 0 "${basename}.gif"
    done
}

main
