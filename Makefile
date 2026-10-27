# Builds, and helpers for the Kindle (docs/kindle-setup.md). KINDLE is the SSH host (~/.ssh/config). Builds run in
# Docker (the eink-rust image).
#
#   make reader          the reader for the Kindle: reader/out/indium (static ARMv7, stripped)
#   make reader-install  build it, copy it and the KOReader plugin to the Kindle (restart KOReader for the plugin)
#   make reader-log      the end of the reader's log on the Kindle
#   make engine          the engine and its tools, native (engine/target/release), for checks on the Mac
#   make bench-armv7     the bench's commands for ARMv7 (bench/bin/armv7)
#   make fetch-blitz     Blitz at its pinned commit with our patches (engine/blitz)
#
#   make kindle-awake    keep it awake: no sleep, so Wi-Fi and SSH stay up (drains the battery: undo when done)
#   make kindle-sleep    let it sleep again (its own power manager, as normal)
#   make kindle-status   awake or not, battery, Wi-Fi, free memory and storage
#   make kindle-wait     wait until it answers over SSH (after waking it by hand)
#   make kindle-ssh      a shell on it

KINDLE ?= kindle
SSH = ssh -o ConnectTimeout=10 $(KINDLE)

DOCKER = docker run --rm -v "$(CURDIR)":/r -v eink-cargo:/cargo -v eink-gomod:/go/pkg/mod \
  -v eink-gocache:/root/.cache/go-build eink-rust

.PHONY: reader reader-install reader-log engine bench-armv7 fetch-blitz
.PHONY: kindle-awake kindle-sleep kindle-status kindle-wait kindle-ssh

reader:
	@sh reader/build.sh

reader-install: reader
	@$(SSH) 'mkdir -p /mnt/us/indium'
	@# a new name, then renamed over the old: overwriting it in place fails now and then on /mnt/us (FUSE)
	@scp -q reader/out/indium $(KINDLE):/mnt/us/indium/indium.new
	@$(SSH) 'mv /mnt/us/indium/indium.new /mnt/us/indium/indium'
	@# a reader kept running is stopped, so the next book starts the new one
	@$(SSH) 'kill $$(cat /var/tmp/indium/pid 2>/dev/null) 2>/dev/null; true'
	@scp -qr reader/indium.koplugin $(KINDLE):/mnt/us/koreader/plugins/
	@echo "reader and plugin copied to $(KINDLE)"

reader-log:
	@$(SSH) 'tail -n 40 /mnt/us/indium/log.txt'

engine:
	@$(DOCKER) sh -c 'cd /r/engine && cargo build --release --features parallel --lib --bins'

bench-armv7:
	@IMAGE=eink-rust bench/go.sh sh build-armv7.sh

fetch-blitz:
	@engine/fetch-blitz.sh

kindle-awake:
	@$(SSH) 'lipc-set-prop com.lab126.powerd preventScreenSaver 1 && lipc-get-prop com.lab126.powerd preventScreenSaver' \
	  | sed 's/^1$$/kept awake (preventScreenSaver 1): run make kindle-sleep when done/'

kindle-sleep:
	@$(SSH) 'lipc-set-prop com.lab126.powerd preventScreenSaver 0 && lipc-get-prop com.lab126.powerd preventScreenSaver' \
	  | sed 's/^0$$/sleeps on its own again (preventScreenSaver 0)/'

kindle-status:
	@$(SSH) 'echo "kept awake: $$(lipc-get-prop com.lab126.powerd preventScreenSaver) (1 yes, 0 no)"; \
	  echo "power: $$(lipc-get-prop com.lab126.powerd status | head -1)"; \
	  echo "battery: $$(lipc-get-prop com.lab126.powerd battLevel)%, $$(lipc-get-prop com.lab126.powerd isCharging | sed "s/1/charging/;s/0/not charging/")"; \
	  echo "wi-fi: $$(lipc-get-prop com.lab126.wifid cmState), $$(ip -4 addr show wlan0 | sed -n "s/.*inet \([0-9.]*\).*/\1/p")"; \
	  echo "memory free: $$(sed -n "s/MemAvailable: *\([0-9]*\) kB/\1/p" /proc/meminfo | awk "{printf \"%d MB\", \$$1/1024}")"; \
	  df -h /mnt/us /var | awk "NR>1 {print \"storage \" \$$6 \": \" \$$4 \" free of \" \$$2}"'

kindle-wait:
	@echo "waiting for $(KINDLE) over SSH (wake it with its power button)..."
	@until $(SSH) true 2>/dev/null; do sleep 5; done; echo "$(KINDLE) is up"

kindle-ssh:
	@$(SSH)
