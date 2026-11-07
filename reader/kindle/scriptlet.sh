#!/bin/sh
# Prints the scriptlet ShamsaInk.sh with its cover in the Icon: line: cover.png (600 x 916, grey, the shape of a book's cover: the opening screen,
# reader/design/golden/kobo-libra-colour/splash-colour-en.png, cropped) in base64.
cd "$(dirname "$0")" || exit 1
icon=$(base64 < cover.png | tr -d '\n')
sed "s|^# Icon:\$|# Icon: data:image/png;base64,$icon|" ShamsaInk.sh
