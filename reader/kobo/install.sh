#!/bin/sh
# From the Mac: a package (reader/scripts/package.sh) installed on the Kobo over SSH: `make kobo-reader-install` (the
# reader just built, packed), `make kobo-package-install` (a package file), `make kobo-release-install` (a release's).
# The same files as a KoboRoot.tgz over USB, put in place differently: unpacked into a folder beside them, then each
# moved in, so a reader that is running keeps the file it started from (a shell reads its script as it goes: writing
# launch.sh in place would break a launcher that is running). Fonts no longer in the package are removed. If the
# reader was open it is restarted on the new build: stopped (launch.sh brings Kobo's interface back), then started
# again as NickelMenu starts it. Not when Bluetooth's daemon runs: leaving the reader would then reboot the Kobo
# (docs/devices/kobo/README.md); nothing is copied either.
# Arguments: the ssh host, the package (its .sha256 beside it is checked when there).
set -e
HOST=$1 PKG=$2
DIR=/mnt/onboard/.adds/shamsaink
NEW=/mnt/onboard/.adds/.shamsaink-new # on the same storage as DIR: a move there is a rename
R="ssh -o ConnectTimeout=10 $HOST"
[ -f "$PKG" ] || { echo "no package $PKG" >&2; exit 1; }
if [ -f "$PKG.sha256" ]; then
  want=$(cut -d' ' -f1 "$PKG.sha256")
  got=$(shasum -a 256 "$PKG" | cut -d' ' -f1)
  [ "$want" = "$got" ] || { echo "$PKG does not match its checksum: not installed" >&2; exit 1; }
fi
open=$($R 'pidof shamsaink >/dev/null && echo yes || echo no')
if [ "$open" = yes ] && [ "$($R 'pidof mtkbtd >/dev/null && echo yes || echo no')" = yes ]; then
  echo "the reader is open and Bluetooth is on: leaving it would reboot the Kobo, so it is not restarted. Leave it yourself, then install again"
  exit 1
fi
$R "rm -rf $NEW && mkdir -p $NEW"
scp -q "$PKG" "$HOST:$NEW/package.tgz"
$R "set -e; cd $NEW && tar -xzf package.tgz && rm package.tgz
  S=$NEW/mnt/onboard/.adds/shamsaink
  mkdir -p $DIR/fonts $DIR/books $DIR/sleep $DIR/licenses /mnt/onboard/.adds/nm
  (cd $DIR/fonts && for f in *; do [ -e \"\$S/fonts/\$f\" ] || rm -f \"\$f\"; done)
  for f in shamsaink launch.sh VERSION getting-started.epub; do mv \$S/\$f $DIR/\$f; done
  for f in \$S/fonts/* \$S/licenses/*; do mv \$f $DIR/\${f#\$S/}; done
  mv $NEW/mnt/onboard/.adds/nm/shamsaink /mnt/onboard/.adds/nm/shamsaink
  rm -rf $NEW $DIR/cache/splash-* $DIR/cache/loading-*"
echo "$(basename "$PKG") installed on $HOST: $($R "cat $DIR/VERSION")"
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
