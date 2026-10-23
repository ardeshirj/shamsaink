# Rust, its 32-bit ARM (ARMv7, musl: static) and arm64 targets, zig as the cross C compiler and linker, and Go (for
# a Go program calling Rust through cgo). Image: eink-rust.
FROM golang:1.27-bookworm AS go
FROM rust:1.95-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends xz-utils file time fonts-dejavu-core \
    && rm -rf /var/lib/apt/lists/*
RUN rustup target add armv7-unknown-linux-musleabihf aarch64-unknown-linux-musl
ARG ZIG=0.15.2
RUN curl -fsSL https://ziglang.org/download/$ZIG/zig-$(uname -m)-linux-$ZIG.tar.xz | tar -xJ -C /opt \
    && ln -s /opt/zig-$(uname -m)-linux-$ZIG/zig /usr/local/bin/zig && zig version
COPY --from=go /usr/local/go /usr/local/go
ENV PATH=/usr/local/go/bin:$PATH CARGO_HOME=/cargo
