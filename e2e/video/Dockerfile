# ffmpeg, to encode the video of the simulated Kobo, and rsvg-convert, to draw the brand's SVGs at the video's sizes
# (e2e/video/README.md): docker build -t eink-ffmpeg e2e/video
FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg librsvg2-bin && rm -rf /var/lib/apt/lists/*
