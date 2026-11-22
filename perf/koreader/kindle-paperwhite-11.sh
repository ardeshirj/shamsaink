# KOReader v2026.07.1 (KMC's kpm package 2026.7.2) on the Kindle Paperwhite 11, firmware 5.19.2: where
# perf/koreader.sh taps (screen pixels: 1236x1648, the touch screen not turned) and how KOReader is started. Calibrated
# from screenshots 2026-11-21 with KOReader's settings at their defaults; perf/koreader.sh sets the menus' remembered
# tabs before each start, so they hold.
SSH=kindle
TMP=/var/tmp                                       # RAM: inktrace while the run lasts
NODE=/dev/input/event1                             # the touch screen (pt_mt)
W=1236 H=1648
touch_xy() { echo $1 $2; }
KO=/mnt/us/koreader                                # KOReader's own folder
BOOK=/mnt/us/shamsaink/books/standard-ebooks/holmes.epub   # the same bytes as ShamsaInk's (copied from it)
# started as its scriptlet in the library starts it (documents/KOReader.sh); Amazon's interface is back once it exits
START="nohup /var/local/kmc/bin/kpm launch koreader --asap >/dev/null 2>&1 </dev/null &"
# the Kindle's own Dark Mode (epdcMode Y8INV: the screen inverts every pixel) off for the run and put back after, as
# devtest does for ShamsaInk (reader/cmd/devtest/smoke.go): with it on, light mode would be drawn inverted
SETUP='m=$(lipc-get-prop -s com.lab126.winmgr epdcMode); case "$m" in *INV) lipc-set-prop com.lab126.winmgr epdcMode "${m%INV}" && echo "$m" > /var/tmp/perf-epdc && echo "the Kindle'"'"'s Dark Mode off for the run";; esac'
RESTORE='[ ! -f /var/tmp/perf-epdc ] || { lipc-set-prop com.lab126.winmgr epdcMode "$(cat /var/tmp/perf-epdc)"; rm /var/tmp/perf-epdc; }'
HOME_UP="! pidof luajit >/dev/null && lipc-get-prop com.lab126.appmgrd activeApp | grep -q KPPMainApp"
BOOK_ROW="516 372"        # the file browser: the book's row (the folder holds only the book)
MENU="618 40"             # the top of the screen: the menu, in the file browser and the book
FM_MAIN="1158 57"         # the file browser's menu: its last tab (the main one)
EXIT_MORE="120 903"       # main tab: Exit >
EXIT="120 375"            # ... Exit
TO_FB="846 57"            # the book's menu: the file browser's icon (the book closed, its state written)
