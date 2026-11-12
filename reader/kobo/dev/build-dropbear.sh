#!/bin/sh
# For development, never in a release: Dropbear (MIT, matt.ucc.asn.au/dropbear), the SSH server of reader/kobo/dev/ssh.sh,
# built static for ARMv7 (musl, Alpine's armv7 image under Docker's emulation) into out/kobo/dropbear, so it runs on
# any Kobo firmware (KOReader's is linked against libcrypt.so.1, which firmware 6.0 has not: 2026-11-11).
# The server and dropbearkey (the host key, made on the Kobo by ssh.sh), no client or scp: the make targets copy with
# `cat >` over ssh. make kobo-dropbear; run from anywhere.
set -eu
cd "$(dirname "$0")/../../.."
V=${DROPBEAR_VERSION:-2026.94}
mkdir -p out/kobo
docker run --rm --platform linux/arm/v7 -v "$PWD/out/kobo:/out" -e V="$V" alpine:3.20 sh -euc '
  apk add -q build-base zlib-dev zlib-static linux-headers
  cd /tmp
  wget -q "https://matt.ucc.asn.au/dropbear/releases/dropbear-$V.tar.bz2"
  sha256sum "dropbear-$V.tar.bz2"
  tar xjf "dropbear-$V.tar.bz2"
  cd "dropbear-$V"
  ./configure --enable-static --disable-lastlog --disable-utmp --disable-utmpx --disable-wtmp --disable-wtmpx \
    --disable-pututline --disable-pututxline > /dev/null
  make -j"$(nproc)" PROGRAMS="dropbear dropbearkey" STATIC=1 > /dev/null
  strip dropbear dropbearkey
  cp dropbear dropbearkey /out/
  file /out/dropbear 2>/dev/null || true
'
ls -l out/kobo/dropbear out/kobo/dropbearkey
