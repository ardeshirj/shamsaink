# Go 1.27 with crengine built from the commit KOReader v2026.07.1 ships (koreader-base 6e4bc81 -> crengine b32a88f),
# installed under /opt/crengine (lib/, include/, data/ with its stylesheets (epub.css imports html5.css) and the hyphenation patterns).
FROM golang:1.27-trixie
RUN apt-get update && apt-get install -y --no-install-recommends cmake g++ pkg-config time \
      libfreetype-dev libharfbuzz-dev libfribidi-dev libunibreak-dev libutf8proc-dev libzstd-dev libpng-dev \
      libjpeg-dev zlib1g-dev libxxhash-dev fonts-dejavu-core \
 && rm -rf /var/lib/apt/lists/*
ARG CRE=b32a88ffbbf505cf2ea5208c1a49002d70d3609f
RUN git clone -q --filter=blob:none https://github.com/koreader/crengine.git /src/crengine && git -C /src/crengine checkout -q $CRE
COPY CMakeLists.txt crsetup.h /src/
RUN cmake -S /src -B /src/build -DCMAKE_BUILD_TYPE=Release && cmake --build /src/build -j"$(nproc)" \
 && mkdir -p /opt/crengine/lib /opt/crengine/data && cp /src/build/libcrengine.a /opt/crengine/lib/ \
 && cp -r /src/crengine/crengine/include /opt/crengine/include && cp /src/crsetup.h /opt/crengine/include/crsetup-ko.h \
 && cp -r /src/crengine/cr3gui/data/*.css /src/crengine/cr3gui/data/hyph /opt/crengine/data/
