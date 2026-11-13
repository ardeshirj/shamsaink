#!/bin/sh
# The third-party licences of what is built into the reader, one file for the packages' licenses/ folder:
# out/licenses/THIRD-PARTY-LICENSES.md (reader/scripts/package.sh copies it). Made from what the build links, not by hand:
#   - the Rust crates of the engine for the devices' target (ARMv7, musl), by cargo-about (reader/licenses/about.toml,
#     about.hbs): each crate's own licence files, else its licence's standard text
#   - the Go modules of the reader (go list, the same tags and target as build-armv7.sh): each one's LICENSE and NOTICE
#   - what the toolchains link in as well: Go's standard library and runtime, musl (the C library, through zig) and
#     zig's compiler-rt, Rust's standard library
# Run from the repository's root wherever build-armv7.sh runs (the eink-rust image: `make licenses`; CI's release job),
# after Blitz is fetched and go-toolkit trimmed. cargo-about is installed the first time (into CARGO_HOME).
#
#     sh reader/scripts/licenses.sh
set -e
cd "$(dirname "$0")/../.."
ABOUT=0.9.3
OUT=${OUT:-out/licenses}
mkdir -p "$OUT"
f=$OUT/THIRD-PARTY-LICENSES.md
[ "$(cargo about --version 2>/dev/null)" = "cargo-about $ABOUT" ] ||
  cargo install -q --locked --features cli cargo-about@$ABOUT
cargo about generate -m engine/Cargo.toml --features parallel \
  -c reader/licenses/about.toml reader/licenses/about.hbs -o "$f" 2>/dev/null

# the Go modules, each with the files its licence asks to be shipped
mods=$(cd reader && CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=7 go list -tags blitz -deps \
  -f '{{with .Module}}{{.Path}}|{{.Version}}|{{.Dir}}{{end}}' . | sort -u | grep -v '^shamsaink')
{
  printf '## The Go modules\n\n'
  printf 'readium/go-toolkit (v0.16.1) is built with its remote archives, fetchers and assets (S3, Google Cloud Storage,\n'
  printf 'HTTP) left out: files removed, none changed (experiments/02-crengine-go/trim-gotoolkit.sh in ShamsaInk'"'"'s repository).\n\n'
  echo "$mods" | while IFS='|' read -r path version dir; do
    [ "$path" = github.com/readium/go-toolkit ] && version=v0.16.1
    files=$(cd "$dir" && ls | grep -E -i '^(licen[cs]e|copying|notice|patents)' || true)
    [ -n "$files" ] || { echo "licenses.sh: no licence file in $path ($dir)" >&2; exit 1; }
    printf '### %s %s\n\n' "$path" "$version"
    for l in $files; do printf '```\n'; cat "$dir/$l"; printf '\n```\n\n'; done
  done
  printf '## Built in by the toolchains\n\n'
  printf '### Go'"'"'s standard library and runtime\n\n```\n'
  cat "$(go env GOROOT)/LICENSE"
  printf '\n```\n\n```\n'
  cat "$(go env GOROOT)/PATENTS"
  printf '\n```\n\n'
  zig=$(dirname "$(readlink -f "$(command -v zig)")")
  printf '### musl, the C library (as zig ships it)\n\n```\n'
  cat "$zig/lib/libc/musl/COPYRIGHT"
  printf '\n```\n\n### zig'"'"'s compiler-rt\n\n```\n'
  cat "$zig/LICENSE"
  printf '\n```\n\n### Rust'"'"'s standard library\n\n'
  printf 'The Rust Standard Library (https://github.com/rust-lang/rust), copyright The Rust Project Developers, used\n'
  printf 'under the Apache License 2.0 (its text above, under the Rust crates) or the MIT License, at your option.\n'
} >> "$f"
echo "$f: $(grep -c '^### ' "$f") licences, $(wc -c < "$f" | tr -d ' ') bytes"
