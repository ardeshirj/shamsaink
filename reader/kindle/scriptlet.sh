#!/bin/sh
# Prints the scriptlet ShamsaInk.sh with its cover in the Icon: line: cover.png (600 x 916, grey, the shape of a book's
# cover) in base64. The cover is ShamsaInk's stacked logo drawn from docs/brand/shamsaink-logo-stack.svg (FarsiEPUB's
# brand/shamsaink/logo-stack.svg), centred on white, in grey: the star about 212 px, far more than half the mark clear
# round it (FarsiEPUB's brand rule for covers, 2026-11-15). Made again after a logo change, rsvg-convert (Debian's
# librsvg2-bin, as reader/ui/splash.go's logos) then ImageMagick (the dpokidov/imagemagick image):
#   rsvg-convert -w 249 -b white shamsaink-logo-stack.svg -o stack.png
#   magick -size 600x916 xc:white stack.png -gravity center -composite -colorspace gray -depth 8 -strip cover.png
cd "$(dirname "$0")" || exit 1
icon=$(base64 < cover.png | tr -d '\n')
sed "s|^# Icon:\$|# Icon: data:image/png;base64,$icon|" ShamsaInk.sh
