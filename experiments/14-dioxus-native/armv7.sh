#!/bin/sh
# Inside eink-rust, from experiments/14-dioxus-native: the spike as a static ARMv7 binary (as reader/build.sh builds
# the engine: musl, rust-lld, NEON), without and with Dioxus; the stripped sizes in out/armv7-*.
set -e
export CARGO_TARGET_DIR=/r/experiments/14-dioxus-native/target
# musl has no libdl.a and Dioxus's hot-patching crate (subsecond, through libloading) asks the linker for -ldl: an
# empty one satisfies it (dlopen itself is in musl's libc.a, and never called here)
mkdir -p /tmp/stub && ar rcs /tmp/stub/libdl.a
export CARGO_TARGET_ARMV7_UNKNOWN_LINUX_MUSLEABIHF_LINKER=rust-lld RUSTFLAGS="-C target-feature=+neon -L /tmp/stub"
T=armv7-unknown-linux-musleabihf
mkdir -p out
cargo build -q --release --target $T
cp target/$T/release/spike out/armv7-templates
cargo build -q --release --target $T --features dioxus
cp target/$T/release/spike out/armv7-dioxus
ls -l out/armv7-templates out/armv7-dioxus | awk '{print $5, $9}'
