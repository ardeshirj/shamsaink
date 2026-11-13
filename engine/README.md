# The engine

ShamsaInk's layout engine in Rust: [Blitz](https://github.com/DioxusLabs/blitz) (Stylo, Taffy, Parley) with our own
pagination (`lib.rs`), glyph layer between Blitz's painter and vello_cpu (`glyphs.rs`), pictures decoded when drawn
(`pictures.rs`), a cache file per book (`saved.rs`), EPUB reading (`epub.rs`) and a C interface for Go (`capi.rs`,
`blitz_engine.h`; the cgo binding is `engine/go`, module `shamsainkengine`). Crate `blitz-engine`, library `blitz_engine`.

Blitz is not in git: `engine/fetch-blitz.sh` clones it into `engine/blitz/` at the pinned commit (21edc92) and applies
`patches/blitz-*.patch` (line culling, pictures sized from their headers, right-to-left list bullets on the right). It also fetches Parley 0.12.0, Blitz's text
layout, from crates.io into `engine/parley/` (its checksum checked) and applies `patches/parley-*.patch` (a line's
trailing space moved to its end, linebender/parley#907, open upstream); `Cargo.toml`'s `[patch.crates-io]` builds that
copy. Safe to run again; stops if `blitz/` or `parley/` is in another state. When #907 is in a Parley release Blitz
uses, the patch and the `[patch]` entry go.

Everything builds in Docker, in the `eink-rust` image (Rust 1.95, arm64 and ARMv7 musl targets, zig, Go;
`experiments/04-rust-engine/docker`), with the registry in the volume `eink-cargo`, from the repo root:

    engine/fetch-blitz.sh
    docker run --rm -v "$PWD":/r -v eink-cargo:/cargo -w /r/engine eink-rust \
      cargo build --release --features parallel --bin blitz-open        # native, both cores

    # static ARMv7 for the devices, NEON on (Rust's target leaves it off), a thread a core with `parallel`
    docker run --rm -v "$PWD":/r -v eink-cargo:/cargo -w /r/engine eink-rust sh -c '
      CARGO_TARGET_ARMV7_UNKNOWN_LINUX_MUSLEABIHF_LINKER=rust-lld RUSTFLAGS="-C target-feature=+neon" \
      CARGO_TARGET_DIR=/r/engine/out/target-neon-par \
      cargo build --release --features parallel --bin blitz-open --target armv7-unknown-linux-musleabihf'

Builds land in `engine/target/` (plain) and `engine/out/target-*` (NEON; both ignored by git).
`experiments/09-kindle/pack.sh` builds every tool for the Kindle and gathers the bundle.

The measuring tools are `src/bin/`: `blitz-pages` (draw every page, times), `blitz-open` (open a book, cache, page
hashes), `blitz-books`, `blitz-demo`, `blitz-profile`, `blitz-show` and `blitz-follow` (a page on the e-ink screen, a
narration followed). History and numbers: `experiments/04-rust-engine/NOTES.md` and 06 to 10's; the whole design:
`docs/DESIGN.md`.
