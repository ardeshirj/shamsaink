#!/bin/sh
# From the Mac (make kobo-reader-install): the reader, its launcher and fonts copied to the Kobo. Each file is written beside
# its place and moved in, so a reader that is running keeps the file it started from (a shell reads its script as it goes:
# writing launch.sh in place would break a launcher that is running). If the reader was open it is restarted on the new
# build: stopped (launch.sh brings Kobo's interface back), then started again as NickelMenu starts it. Not when Bluetooth's
# daemon runs: leaving the reader would then reboot the Kobo (docs/devices/kobo/README.md); the files are not copied either.
# Arguments: the ssh host, the folder on the Kobo.
set -e
HOST=$1 DIR=$2
R="ssh -o ConnectTimeout=10 $HOST"
open=$($R 'pidof shamsaink >/dev/null && echo yes || echo no')
if [ "$open" = yes ] && [ "$($R 'pidof mtkbtd >/dev/null && echo yes || echo no')" = yes ]; then
  echo "the reader is open and Bluetooth is on: leaving it would reboot the Kobo, so it is not restarted. Leave it yourself, then install again"
  exit 1
fi
KEEP=$(cd reader/out/fonts && echo *.ttf) # fonts no longer in the folder are removed from the device
$R "mkdir -p $DIR/fonts $DIR/books $DIR/sleep"
scp -q reader/out/shamsaink "$HOST:$DIR/shamsaink.new"
scp -q reader/kobo/launch.sh "$HOST:$DIR/launch.sh.new"
scp -q reader/out/fonts/* "$HOST:$DIR/"
$R "cd $DIR && mv shamsaink.new shamsaink && mv launch.sh.new launch.sh && mv *.ttf fonts/ && (cd fonts && for f in *.ttf; do case \" $KEEP \" in *\" \$f \"*) ;; *) rm -f \$f;; esac; done) && rm -f cache/splash-*.gray"
echo "reader and launcher copied to $HOST"
if [ "$open" = yes ]; then
  echo "the reader was open: restarting it on the new build"
  $R 'kill $(pidof shamsaink)'
  i=0
  while [ $i -lt 60 ]; do # Kobo's interface back (its launcher starts it as the reader ends)
    sleep 2
    $R 'pidof nickel >/dev/null' 2>/dev/null && break
    i=$((i + 1))
  done
  sleep 8 # Nickel settled: the launcher stops it again
  $R "cd $DIR && nohup sh launch.sh >/dev/null 2>&1 </dev/null &"
  echo "started again"
fi
