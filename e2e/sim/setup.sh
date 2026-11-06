#!/bin/sh
# Makes a fresh simulated Kobo in DIR (docs/TESTING.md, section 2): its hardware files, as the reader reads them under
# SHAMSAINK_SIM, and the reader's folder as reader/kobo/install.sh leaves it on the Kobo, with BOOKS (a folder: copied
# as the reader's books/, its files dated 2026-01-01 so the library's "added" is the same every run). The scenario's
# fixture is laid by devtest, as on the Kobo.
#     sh e2e/sim/setup.sh DIR READER FONTS BOOKS
set -e
DIR=$1 READER=$2 FONTS=$3 BOOKS=$4
[ -d "$BOOKS" ] || { echo "usage: setup.sh DIR READER FONTS BOOKS" >&2; exit 2; }
mkdir -p "$DIR" && find "$DIR" -mindepth 1 -delete # emptied, not removed: CI makes /sim once, as root, for its user
mkdir -p "$DIR"/dev/input "$DIR"/proc/bus/input "$DIR"/tmp "$DIR"/mnt/onboard/.kobo \
  "$DIR"/sys/class/power_supply/bd71827_bat "$DIR"/sys/class/backlight/mxc_msp430.0 "$DIR"/sys/class/backlight/lm3630a_led

# the screen: the Kobo's framebuffer, white (the reader maps it)
head -c $((5056 * 1680)) /dev/zero | tr '\0' '\377' > "$DIR"/dev/fb0
# the touch screen, the power key and the page buttons: named pipes in the input-event format
mkfifo "$DIR"/dev/input/event1 "$DIR"/dev/input/event3 "$DIR"/dev/input/event0
cat > "$DIR"/proc/bus/input/devices <<'EOF'
I: Bus=0019 Vendor=0001 Product=0001 Version=0100
N: Name="gpio-keys"
H: Handlers=event0

EOF
# the battery, the light and its warmth; the Kobo Libra Colour's product id
echo 80 > "$DIR"/sys/class/power_supply/bd71827_bat/capacity
echo Discharging > "$DIR"/sys/class/power_supply/bd71827_bat/status
echo 0 > "$DIR"/sys/class/backlight/mxc_msp430.0/brightness
echo 30 > "$DIR"/sys/class/backlight/mxc_msp430.0/actual_brightness
echo 5 > "$DIR"/sys/class/backlight/lm3630a_led/color
echo "N000000000000,4.9.77,4.46.23852,4.9.77,4.9.77,00000000-0000-0000-0000-000000000390" > "$DIR"/mnt/onboard/.kobo/version

# the reader's folder
R="$DIR"/mnt/onboard/.adds/shamsaink
mkdir -p "$R"/fonts "$R"/logs "$R"/sleep
cp "$READER" "$R"/shamsaink
cp "$(dirname "$0")"/launch.sh "$R"/launch.sh
cp "$FONTS"/*.ttf "$R"/fonts/
cp -R "$BOOKS" "$R"/books
find "$R"/books -exec touch -d "2026-01-01 09:00:00" {} +
