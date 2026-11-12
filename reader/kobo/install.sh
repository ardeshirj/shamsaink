#!/bin/sh
# From the Mac: a package (reader/scripts/package.sh) installed on the Kobo over SSH: `make kobo-reader-install` (the
# reader just built, packed), `make kobo-package-install` (a package file), `make kobo-release-install` (a release's).
# The same files as the installer copies over USB, put in place differently: unpacked into a folder beside them, then each
# moved in, so a reader that is running keeps the file it started from (a shell reads its script as it goes: writing
# launch.sh in place would break a launcher that is running). Fonts and licences no longer in the package are removed.
# The launcher (launcher/Kobo.tgz in the package) is put in /usr/local/Kobo/imageformats/ the same way, moved in, when it
# differs: Nickel loads it at its next start, its start-up guard checking it then. If the
# reader was open it is restarted on the new build: stopped (launch.sh brings Kobo's interface back), then started
# again as the launcher's row starts it. Sent through ssh, not scp: the Mac's scp needs an SFTP server, which our
# development Dropbear has not (reader/kobo/dev/).
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
$R "rm -rf $NEW && mkdir -p $NEW"
$R "cat > $NEW/package.tgz" < "$PKG"
$R "set -e; cd $NEW && tar -xzf package.tgz && rm package.tgz
  S=$NEW/mnt/onboard/.adds/shamsaink
  mkdir -p $DIR/fonts $DIR/books $DIR/sleep $DIR/licenses
  (cd $DIR/fonts && for f in *; do [ -e \"\$S/fonts/\$f\" ] || rm -f \"\$f\"; done)
  (cd $DIR/licenses && for f in *; do [ -e \"\$S/licenses/\$f\" ] || rm -f \"\$f\"; done)
  if [ -f \$S/launcher/Kobo.tgz ]; then
    mkdir -p $DIR/launcher && mv \$S/launcher/Kobo.tgz $DIR/launcher/Kobo.tgz
    L=/usr/local/Kobo/imageformats/libshamsaink-launcher.so
    tar -xzOf $DIR/launcher/Kobo.tgz imageformats/libshamsaink-launcher.so > \$L.new
    if cmp -s \$L.new \$L; then rm \$L.new; else chmod 755 \$L.new && mv \$L.new \$L && echo 'launcher: new, loaded when Nickel next starts'; fi
  fi
  for f in shamsaink launch.sh VERSION getting-started.epub; do mv \$S/\$f $DIR/\$f; done
  for f in \$S/fonts/* \$S/licenses/*; do mv \$f $DIR/\${f#\$S/}; done
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
