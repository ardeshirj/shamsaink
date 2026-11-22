# KOReader v2026.07.1 (kobov5) on the Kobo Libra Colour, firmware 6.0: where perf/koreader.sh taps (screen pixels, the
# screen as the reader shows it: 1264x1680) and how KOReader is started. Calibrated from screenshots 2026-11-21 with
# KOReader's settings at their defaults; perf/koreader.sh sets the menus' remembered tabs before each start, so they hold.
SSH=kobo
TMP=/tmp                                           # RAM: inktrace while the run lasts
NODE=/dev/input/event1                             # the touch screen
W=1264 H=1680
touch_xy() { echo $((H - $2)) $1; }                # the touch screen is turned: touch x = h - y, touch y = x
KO=/mnt/onboard/.adds/koreader                     # KOReader's own folder
BOOK=/mnt/onboard/.adds/shamsaink/books/standard-ebooks/holmes.epub   # the same bytes as ShamsaInk's (copied from it)
# started as ShamsaInk is, by its launch.sh (development only: --run, a script in dev/readers/, put there for the run
# from perf/koreader/kobo-start.sh and removed after); Kobo's interface is back once it exits
RUNNER_LOCAL=perf/koreader/kobo-start.sh
RUNNER=/mnt/onboard/.adds/shamsaink/dev/readers/KOReader.sh
START="cd /mnt/onboard/.adds/shamsaink && nohup sh launch.sh --run $RUNNER >/dev/null 2>&1 </dev/null &"
HOME_UP="pidof nickel >/dev/null && ! pidof luajit >/dev/null"
BOOK_ROW="525 375"        # the file browser: the book's row (the folder holds only the book)
MENU="632 40"             # the top of the screen: the menu, in the file browser and the book
FM_MAIN="1188 57"         # the file browser's menu: its last tab (the main one)
EXIT_MORE="120 1035"      # main tab: Exit >
EXIT="120 600"            # ... Exit
TO_FB="870 57"            # the book's menu: the file browser's icon (the book closed, its state written)
