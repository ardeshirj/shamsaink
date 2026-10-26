#!/bin/sh
# On the Mac: builds the static 32-bit ARM binaries and gathers them in out/bundle, to copy to the device, with the
# books (as they are), the fallback fonts and KOReader's reMarkable release (fetched once, checksum checked):
#   run.sh:       pages (the Go engine, experiment 03's, through the shared interface) and blitz-pages (04, patched)
#   run-neon.sh:  blitz-pages built with NEON, blitz-profile with and without
#   run-ko.sh:    KOReader's own crengine, through 01's crebench.lua
#   run-scene.sh: blitz-profile with NEON and blitz-knobs.patch (applied to the Blitz clone for that build only)
# Needs eink-rust, Blitz fetched (engine/fetch-blitz.sh), and the books of experiments 03 and 05.
set -e
cd "$(dirname "$0")"
X=$(cd .. && pwd)
R=$(cd ../.. && pwd)
O=out/bundle
rm -rf $O && mkdir -p $O/bin $O/books $O/fonts
docker run --rm -v "$X":/x -v eink-cargo:/cargo -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build -v "$R":/r \
  -w /x eink-rust sh -c '
  export CARGO_TARGET_ARMV7_UNKNOWN_LINUX_MUSLEABIHF_LINKER=rust-lld
  (cd /r/engine &&
    cargo build -q --release --bin blitz-pages --bin blitz-profile --target armv7-unknown-linux-musleabihf &&
    RUSTFLAGS="-C target-feature=+neon" CARGO_TARGET_DIR=/r/engine/out/target-neon \
      cargo build -q --release --bin blitz-pages --bin blitz-profile --target armv7-unknown-linux-musleabihf &&
    cd blitz && git apply /x/06-device-timing/blitz-knobs.patch && cd .. &&
    RUSTFLAGS="-C target-feature=+neon" CARGO_TARGET_DIR=/r/engine/out/target-knobs \
      cargo build -q --release --bin blitz-profile --target armv7-unknown-linux-musleabihf;
    s=$?; (cd blitz && git apply -R /x/06-device-timing/blitz-knobs.patch); exit $s)
  (cd engine && CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags "-s -w" \
    -o /x/06-device-timing/out/bundle/bin/ ./cmd/pages)'
T=armv7-unknown-linux-musleabihf/release
cp ../../engine/target/$T/blitz-pages ../../engine/target/$T/blitz-profile $O/bin/
cp ../../engine/out/target-neon/$T/blitz-pages $O/bin/blitz-pages-neon
cp ../../engine/out/target-neon/$T/blitz-profile $O/bin/blitz-profile-neon
cp ../../engine/out/target-knobs/$T/blitz-profile $O/bin/blitz-profile-knobs
cp ../03-own-layout/out/cmp/buried-alive/book.epub $O/books/buried-alive.epub
cp -R ../03-own-layout/out/cmp/buried-alive/src $O/books/buried-alive
cp ../05-real-css/book/holmes.epub $O/books/holmes.epub
cp -R ../05-real-css/book/src $O/books/holmes
cp ../05-real-css/out/fonts/*.ttf $O/fonts/
cp run.sh run-neon.sh run-ko.sh run-scene.sh ../01-renderers/koreader/crebench.lua $O/
KO=v2026.07.1 SHA=f3576f15956f4cdcb08df81ecd85236d68f63d2791209672aca12a7981668daa
if [ ! -d out/ko/koreader ]; then
  mkdir -p out/ko && curl -sSLo out/ko/ko-rm.zip \
    https://github.com/koreader/koreader/releases/download/$KO/koreader-remarkable-$KO.zip
  echo "$SHA  out/ko/ko-rm.zip" | shasum -a 256 -c - && (cd out/ko && unzip -q ko-rm.zip)
fi
cp -R out/ko/koreader $O/
file $O/bin/* | sed 's/,.*stripped//'
du -sh $O
