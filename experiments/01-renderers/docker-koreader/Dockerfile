# KOReader's Linux arm64 release (native on Apple silicon), for driving its engine (crengine) headless.
FROM debian:bookworm-slim
ARG KO=v2026.07.1
ARG SHA=68b399a99a88bf3f036f5e8b67d043215a28e11d375cf0823900aef7b1523a0b
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl xz-utils time zip unzip && rm -rf /var/lib/apt/lists/* \
 && curl -sSLo /tmp/ko.tar.xz https://github.com/koreader/koreader/releases/download/$KO/koreader-linux-arm64-$KO.tar.xz \
 && echo "$SHA  /tmp/ko.tar.xz" | sha256sum -c - \
 && mkdir /opt/ko && tar xf /tmp/ko.tar.xz -C /opt/ko && rm /tmp/ko.tar.xz
WORKDIR /opt/ko/lib/koreader
