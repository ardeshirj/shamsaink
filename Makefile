# Builds, and helpers for the devices, in three parts: development, the Kobo, the Kindle. Builds run in Docker (the
# eink-rust image).

DOCKER = docker run --rm -v "$(CURDIR)":/r -v eink-cargo:/cargo -v eink-gomod:/go/pkg/mod \
  -v eink-gocache:/root/.cache/go-build eink-rust

# --- Development ---
#
#   make reader          the reader: reader/out/shamsaink (static ARMv7, stripped), for both devices
#   make engine          the engine and its tools, native (engine/target/release), for checks on the Mac
#   make bench-armv7     the bench's commands for ARMv7 (bench/bin/armv7)
#   make fetch-blitz     Blitz at its pinned commit with our patches (engine/blitz)
#   make design          the design system checked (docs/design-system.md): the spec against reader/ui, and every screen
#                        drawn and compared with its kept render (needs `make engine`); `make design GOLDEN=update` keeps
#                        the drawings as the new renders; writes ui.css's variables, ui/tokens_gen.go and docs/design/system.html, the page of it all

.PHONY: reader engine bench-armv7 fetch-blitz design

reader:
	@sh reader/build.sh

engine:
	@$(DOCKER) sh -c 'cd /r/engine && cargo build --release --features parallel --lib --bins'

bench-armv7:
	@IMAGE=eink-rust bench/go.sh sh build-armv7.sh

fetch-blitz:
	@engine/fetch-blitz.sh

design:
	@$(DOCKER) sh -c 'cd /r/reader && go run ./design/cmd/gen tokens && FONTS=/r/reader/out/fonts GOLDEN=$(GOLDEN) go test -count=1 -tags blitz ./ui -run "TestEveryScreen|TestScreenRenders" && go run ./design/cmd/gen page && go test -count=1 ./design'

# --- The Kobo ---
#
#   make kobo-check      the Kobo held against the design spec's profile of it (reader/cmd/devtest, read-only: screen,
#                        touch, fonts by hash, the reader installed); the tool is built for the Mac with Docker
# Experiment 13, docs/devices/kobo/setup.md; KOBO is its SSH host (KOReader's SSH server, which stays up after KOReader is
# closed).
#   make kobo-reader-install  build the reader, copy it and its launcher to .adds/shamsaink (refused while a book is
#                        open: the launcher is read by its shell as it runs)
#   make kobo-reader-log the end of the reader's log on the Kobo, and when the Kobo was last up while reading
#   make kobo-status     its sleep timer, battery, Wi-Fi, free memory and storage
#   make kobo-wait       wait until it answers over SSH (after waking it by hand)
#   make kobo-ssh        a shell on it
#   make kobo-nickelmenu NickelMenu (NM_VERSION) installed again over USB, e.g. after a crash removed it (downloaded
#                        once into out/); the Kobo is ejected: unplug it
#   make kobo-import     books (re)imported: the Kobo, connected over USB, is cleaned of `._` files and ejected
#   make e2e             an end-to-end test on the Kobo (e2e/run.py): SCENARIO (e2e/kobo-basics.txt by default), its
#                        pages compared with the references kept by a run with SAVE=1 (in e2e/out/, not committed)

KOBO ?= kobo
KOBO_SSH = ssh -o ConnectTimeout=10 $(KOBO)
# The reader on the Kobo, started from NickelMenu (docs/devices/kobo/setup.md, section 7): in .adds/shamsaink, the program
# named shamsaink there, as in reader/out
KOBO_DIR = /mnt/onboard/.adds/shamsaink
# Kobo's settings: its sleep timer is [PowerOptions] AutoSleepMinutes (0: never). Kobo's interface reads them only
# when it starts, so the timer is set by hand in its Settings (docs/devices/kobo/setup.md, section 4), not here.
KOBO_CONF = "/mnt/onboard/.kobo/Kobo/Kobo eReader.conf"
NM_VERSION ?= 0.6.0
NM_TGZ = out/nickelmenu-$(NM_VERSION)-KoboRoot.tgz
KOBO_USB = /Volumes/KOBOeReader

.PHONY: kobo-check kobo-reader-install kobo-reader-log kobo-status kobo-wait kobo-ssh kobo-nickelmenu kobo-import e2e

devtest:
	@docker run --rm -v "$(CURDIR)":/r -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build -w /r/reader eink-go \
	  sh -c 'CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o out/devtest ./cmd/devtest'

kobo-check: devtest
	@reader/out/devtest check kobo-libra-colour

kobo-reader-install: reader
	@$(KOBO_SSH) 'pidof shamsaink >/dev/null && { echo "a book is open on the Kobo: close it first"; exit 1; }; \
	  mkdir -p $(KOBO_DIR)/fonts $(KOBO_DIR)/books $(KOBO_DIR)/sleep'
	@scp -q reader/out/shamsaink $(KOBO):$(KOBO_DIR)/shamsaink.new
	@scp -q reader/kobo/launch.sh reader/out/fonts/* $(KOBO):$(KOBO_DIR)/
	@$(KOBO_SSH) 'cd $(KOBO_DIR) && mv shamsaink.new shamsaink && mv *.ttf fonts/ && rm -f cache/splash-*.gray'
	@echo "reader and launcher copied to $(KOBO)"

SCENARIO ?= e2e/kobo-basics.txt
e2e:
	@e2e/run.py $(if $(SAVE),--save) kobo $(SCENARIO)

kobo-reader-log:
	@$(KOBO_SSH) 'tail -n 40 $(KOBO_DIR)/logs/reader.log; echo "last up while reading:"; \
	  tail -n 1 $(KOBO_DIR)/logs/alive.log'

kobo-status:
	@$(KOBO_SSH) 'B=/sys/class/power_supply/bd71827_bat; \
	  echo "sleeps after: $$(sed -n "s/^AutoSleepMinutes=//p" $(KOBO_CONF)) minutes (0: never)"; \
	  echo "battery: $$(cat $$B/capacity 2>/dev/null)%, $$(cat $$B/status 2>/dev/null)"; \
	  echo "wi-fi: $$(ip -4 addr show wlan0 2>/dev/null | sed -n "s/.*inet \([0-9.]*\).*/\1/p")"; \
	  echo "memory free: $$(sed -n "s/MemAvailable: *\([0-9]*\) kB/\1/p" /proc/meminfo | awk "{printf \"%d MB\", \$$1/1024}")"; \
	  df -h /mnt/onboard /tmp | awk "NR>1 {print \"storage \" \$$6 \": \" \$$4 \" free of \" \$$2}"'

kobo-wait:
	@echo "waiting for $(KOBO) over SSH (wake it with its power button; never answers: open KOReader)..."
	@until $(KOBO_SSH) true 2>/dev/null; do sleep 5; done; echo "$(KOBO) is up"

kobo-ssh:
	@$(KOBO_SSH)

# Over USB: without NickelMenu, KOReader (and its SSH server) cannot be started. Kobo installs .kobo/KoboRoot.tgz
# once unplugged (and deletes it).
kobo-nickelmenu: $(NM_TGZ)
	@[ -d $(KOBO_USB)/.kobo ] || { echo "the Kobo is not connected: plug it in and tap Connect"; exit 1; }
	@cp $(NM_TGZ) $(KOBO_USB)/.kobo/KoboRoot.tgz && dot_clean -m $(KOBO_USB) && diskutil eject $(KOBO_USB) >/dev/null
	@echo "NickelMenu $(NM_VERSION) copied, Kobo ejected: unplug it, it installs and restarts (about a minute)"

# Kobo imports new books and drops the gone ones (and what ExcludeSyncFolders now skips) when a USB connection ends.
kobo-import:
	@[ -d $(KOBO_USB)/.kobo ] || { echo "the Kobo is not connected: plug it in and tap Connect"; exit 1; }
	@dot_clean -m $(KOBO_USB) && diskutil eject $(KOBO_USB) >/dev/null
	@echo "Kobo ejected: unplug it, it imports its books (a minute or so)"

$(NM_TGZ):
	@mkdir -p out
	@curl -fsSL -o $@ https://github.com/pgaskin/NickelMenu/releases/download/v$(NM_VERSION)/KoboRoot.tgz

# --- The Kindle ---
#
# docs/devices/kindle/setup.md; KINDLE is its SSH host (~/.ssh/config).
#   make kindle-reader-install  build the reader, copy it and the KOReader plugin to the Kindle (restart KOReader for
#                        the plugin; reader-install until 2026-10-30)
#   make kindle-reader-log  the end of the reader's log on the Kindle (reader-log until 2026-10-30)
#   make kindle-awake    keep it awake: no sleep, so Wi-Fi and SSH stay up (drains the battery: undo when done)
#   make kindle-sleep    let it sleep again (its own power manager, as normal)
#   make kindle-status   awake or not, battery, Wi-Fi, free memory and storage
#   make kindle-wait     wait until it answers over SSH (after waking it by hand)
#   make kindle-ssh      a shell on it

KINDLE ?= kindle
SSH = ssh -o ConnectTimeout=10 $(KINDLE)

.PHONY: kindle-reader-install kindle-reader-log kindle-awake kindle-sleep kindle-status kindle-wait kindle-ssh

kindle-reader-install: reader
	@$(SSH) 'mkdir -p /mnt/us/shamsaink'
	@# a new name, then renamed over the old: overwriting it in place fails now and then on /mnt/us (FUSE)
	@scp -q reader/out/shamsaink $(KINDLE):/mnt/us/shamsaink/shamsaink.new
	@$(SSH) 'mv /mnt/us/shamsaink/shamsaink.new /mnt/us/shamsaink/shamsaink'
	@# a reader kept running is stopped, so the next book starts the new one
	@$(SSH) 'kill $$(cat /var/tmp/shamsaink/pid 2>/dev/null) 2>/dev/null; true'
	@scp -qr reader/shamsaink.koplugin $(KINDLE):/mnt/us/koreader/plugins/
	@echo "reader and plugin copied to $(KINDLE)"

kindle-reader-log:
	@$(SSH) 'tail -n 40 /mnt/us/shamsaink/log.txt'

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
