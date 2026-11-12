#!/bin/sh
# The Kobo launcher for firmware 5.x/6.x (plugin.cpp) built into out/kobo/Kobo.tgz: imageformats/libshamsaink-launcher.so,
# which the firmware unpacks under /usr/local/Kobo. In kobuild's public Qt 6 image (ghcr.io/nicoverbruggen/kobuild,
# MIT), in Docker. make kobo-launcher; run from anywhere.
set -eu
cd "$(dirname "$0")/../../.."
image=${KOBUILD_QT6:-ghcr.io/nicoverbruggen/kobuild:qt6-v0.1.0}
mkdir -p out/kobo
docker run --rm -v "$PWD:/r" -w /r "$image" sh -euc '
  b=out/kobo/launcher-build
  cmake -S reader/kobo/launcher -B $b/cmake -G Ninja -DCMAKE_BUILD_TYPE=Release -DCMAKE_TOOLCHAIN_FILE="$CMAKE_TOOLCHAIN_FILE"
  cmake --build $b/cmake
  stage=$b/stage; rm -rf "$stage"; mkdir -p "$stage/imageformats"
  cp $b/cmake/libshamsaink-launcher.so "$stage/imageformats/"
  arm-linux-gnueabihf-strip --strip-unneeded "$stage/imageformats/libshamsaink-launcher.so"
  chmod 755 "$stage/imageformats/libshamsaink-launcher.so"
  tar --owner=0 --group=0 --numeric-owner -czf out/kobo/Kobo.tgz -C "$stage" imageformats/libshamsaink-launcher.so
  tar tzvf out/kobo/Kobo.tgz
'
