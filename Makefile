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
#   make check           seconds, no engine, what CI runs (reader/scripts/check.sh): gofmt, vet, and the tests of every package
#                        that needs no engine: the design spec, the device layer, devtest, the library (a pre-push hook
#                        runs it: `git config core.hooksPath .githooks`, once per clone)
#   make test            every Go test with the engine (needs `make engine`): make check's, the reader's own and
#                        reader/ui's, and every screen drawn and compared with its kept render; writes nothing
#   make kobo-package    the reader built and packed for the Kobo (reader/scripts/package.sh): out/release/shamsaink-VERSION-kobo.tgz
#                        and its .sha256, VERSION git's description of this commit; the same package a release has
#   make kindle-package  the same for the Kindle: out/release/shamsaink-VERSION-kindle.tgz and its .sha256
#   make tag V=0.0.1 NOTE="…"  tag a release on a clean main (reader/scripts/release.sh); push it yourself, and CI
#                        builds and publishes it (CHANGELOG.md, "How to release")
#   make design-update   after a change to the design made on purpose: writes ui.css's variables, ui/tokens_gen.go, the
#                        kept renders (reader/design/golden) and docs/design/system.html; look at the renders' diff before
#                        committing (docs/DESIGN-SYSTEM.md)

.PHONY: reader welcome engine bench-armv7 fetch-blitz design-update check test kobo-package kindle-package tag

reader:
	@sh reader/scripts/build.sh

# the version as build.sh writes it into the binary
VERSION = $(shell git describe --tags --match 'v[0-9]*' --dirty --always)

# the getting-started guide as an EPUB (reader/scripts/welcome.sh: pandoc in Docker), for the packages
welcome:
	@sh reader/scripts/welcome.sh

kobo-package: reader welcome
	@$(DOCKER) sh -c 'cd /r && VERSION=$(VERSION) sh reader/scripts/package.sh'

kindle-package: reader welcome
	@$(DOCKER) sh -c 'cd /r && VERSION=$(VERSION) BRAND=kindle sh reader/scripts/package.sh'

tag:
	@sh reader/scripts/release.sh "$(V)" "$(NOTE)"

engine:
	@$(DOCKER) sh -c 'cd /r/engine && cargo build --release --features parallel --lib --bins'

bench-armv7:
	@IMAGE=eink-rust bench/go.sh sh build-armv7.sh

fetch-blitz:
	@engine/fetch-blitz.sh

test:
	@$(DOCKER) sh -c 'cd /r/reader && sh scripts/check.sh && FONTS=/r/reader/fonts go test -count=1 -tags blitz ./...'

design-update:
	@$(DOCKER) sh -c 'cd /r/reader && go run ./design/cmd/gen tokens && FONTS=/r/reader/fonts GOLDEN=update go test -count=1 -tags blitz ./ui -run "TestEveryScreen|TestScreenRenders" && go run ./design/cmd/gen page && go test -count=1 ./design'

# The same as .github/workflows/ci.yml (reader/scripts/check.sh), so a push that CI would fail is stopped here.
check:
	@docker run --rm -v "$(CURDIR)":/r -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build eink-go sh -c 'sh /r/reader/scripts/check.sh'

# --- The Kobo ---
#
#   make kobo-check      the Kobo held against the design spec's profile of it (reader/cmd/devtest, read-only: screen,
#                        touch, fonts by hash, the reader installed); the tool is built for the Mac with Docker
#   make kobo-reader-install  build the reader, pack it (make kobo-package) and install that package over SSH
#                        (reader/kobo/install.sh: files moved in whole, so an open reader is not hurt); if the reader was
#                        open it is restarted on the new build, unless Bluetooth is on (leaving would reboot the Kobo: then
#                        nothing is copied and it says so)
#   make kobo-package-install PKG=file  a package file installed the same way (its .sha256 beside it checked)
#   make kobo-release-install V=0.0.1  a release's package downloaded from GitHub (into out/download; with `gh` while the
#                        repository is private, curl once public), checked and installed the same way
#   make kobo-reader-log the end of the reader's log on the Kobo, and when the Kobo was last up while reading
#   make kobo-usage      the reader's usage lines (start, stop, sleep, wake with the battery) as a table: awake and asleep stretches
#                        and the battery's use per hour
#   make kobo-status     its sleep timer, battery, Wi-Fi, free memory and storage
#   make kobo-wait       wait until it answers over SSH (after waking it by hand)
#   make kobo-ssh        a shell on it
#   make kobo-nickelmenu NickelMenu (NM_VERSION) installed again over USB, e.g. after a crash removed it (downloaded
#                        once into out/); the Kobo is ejected: unplug it
#   make kobo-import     books (re)imported: the Kobo, connected over USB, is cleaned of `._` files and ejected
#   make e2e-sim         the scenarios on the simulated Kobo (e2e/README.md), in Docker: SCENARIO (every one marked on: sim by
#                        default), each on a fresh device; SIM_BOOKS (out/sim-books) is the books/ folder it gets
#   make e2e             a scenario on the Kobo (e2e/README.md): SCENARIO (e2e/scenarios/basics.txt by default), by
#                        control name, its pages compared with the references kept by a run with SAVE=1 (in e2e/out/, not
#                        committed). Both: RUNS=5 plays the list five times (the Kobo stops at a failure)

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

.PHONY: kobo-check kindle-check kobo-reader-install kobo-package-install kobo-release-install kobo-reader-log kobo-usage kobo-status kobo-wait kobo-ssh kobo-dev-ssh kobo-nickelmenu kobo-import e2e e2e-sim

devtest:
	@docker run --rm -v "$(CURDIR)":/r -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build -w /r/reader eink-go \
	  sh -c 'CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o out/devtest ./cmd/devtest'

kobo-check: devtest
	@reader/out/devtest check kobo-libra-colour

kindle-check: devtest
	@reader/out/devtest check kindle-paperwhite-11

kobo-reader-install: kobo-package
	@sh reader/kobo/install.sh $(KOBO) out/release/shamsaink-$(VERSION)-kobo.tgz

kobo-package-install:
	@[ -n "$(PKG)" ] || { echo "make kobo-package-install PKG=out/release/shamsaink-…-kobo.tgz"; exit 1; }
	@sh reader/kobo/install.sh $(KOBO) $(PKG)

REPO = ardeshirj/shamsaink
kobo-release-install:
	@[ -n "$(V)" ] || { echo "make kobo-release-install V=0.0.1"; exit 1; }
	@f=shamsaink-v$(V:v%=%)-kobo.tgz; mkdir -p out/download; \
	  if [ ! -f out/download/$$f ]; then \
	    if command -v gh >/dev/null; then gh release download v$(V:v%=%) -R $(REPO) -p "$$f*" -D out/download; \
	    else for x in $$f $$f.sha256; do curl -fsSL -o out/download/$$x \
	      https://github.com/$(REPO)/releases/download/v$(V:v%=%)/$$x || { rm -f out/download/$$f*; \
	      echo "not downloaded: while the repository is private, install gh (brew install gh; gh auth login)"; exit 1; }; done; fi; \
	  fi; sh reader/kobo/install.sh $(KOBO) out/download/$$f

kobo-usage: devtest
	@reader/out/devtest usage kobo-libra-colour

RUNS ?= 1
runs = $(foreach i,$(shell seq $(RUNS)),$(1))
# DEVICE=kindle runs them on the Kindle instead (its profile kindle-paperwhite-11); a scenario not on: it is skipped.
E2E_DEVICE = $(if $(filter kindle,$(DEVICE)),kindle-paperwhite-11,kobo-libra-colour)
e2e: devtest
	@for s in $(call runs,$(or $(SCENARIO),e2e/scenarios/basics.txt)); do \
	  echo "== $$s ($(E2E_DEVICE))"; reader/out/devtest scenario $(E2E_DEVICE) $$s $(if $(SAVE),--save) || exit 1; done

SIM_BOOKS ?= out/sim-books
e2e-sim:
	@$(DOCKER) sh /r/e2e/sim/run.sh $(SIM_BOOKS) $(if $(SAVE),--save) $(call runs,$(or $(SCENARIO),$(wildcard e2e/scenarios/*.txt)))

kobo-reader-log:
	@$(KOBO_SSH) 'tail -n 40 $(KOBO_DIR)/logs/reader.log; echo "last up while reading:"; \
	  tail -n 1 $(KOBO_DIR)/logs/alive.log'

kobo-status:
	@$(KOBO_SSH) 'B=/sys/class/power_supply/bd71827_bat; \
	  echo "firmware: $$(cut -d, -f3 /mnt/onboard/.kobo/version)"; \
	  echo "shamsaink: $$(cat /mnt/onboard/.adds/shamsaink/VERSION 2>/dev/null || echo not installed)"; \
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

# For development: NickelMenu's "Toggle SSH" (reader/kobo/dev/), never in the reader's package. Over USB when the Kobo
# is connected (the first time: no SSH yet), else over SSH.
kobo-dev-ssh:
	@if [ -d $(KOBO_USB)/.adds ]; then \
	  mkdir -p $(KOBO_USB)/.adds/shamsaink/dev $(KOBO_USB)/.adds/nm && \
	  cp reader/kobo/dev/ssh.sh $(KOBO_USB)/.adds/shamsaink/dev/ssh.sh && \
	  cp reader/kobo/dev/nm-ssh $(KOBO_USB)/.adds/nm/ssh && \
	  dot_clean -m $(KOBO_USB) && diskutil eject $(KOBO_USB) >/dev/null && \
	  echo "Toggle SSH copied, Kobo ejected: in NickelMenu once unplugged"; \
	else \
	  $(KOBO_SSH) 'mkdir -p $(KOBO_DIR)/dev /mnt/onboard/.adds/nm && cat > $(KOBO_DIR)/dev/ssh.sh' < reader/kobo/dev/ssh.sh && \
	  $(KOBO_SSH) 'cat > /mnt/onboard/.adds/nm/ssh' < reader/kobo/dev/nm-ssh && \
	  echo "Toggle SSH installed on $(KOBO): in NickelMenu (its menu reread at Nickel's next start)"; \
	fi

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
#   make kindle-check    the Kindle held against its design profile (as kobo-check); make e2e DEVICE=kindle: scenarios on it
#   make kindle-reader-install  build the reader, copy it, its fonts, the launcher and the scriptlet to the Kindle
#                        (KOReader's plugin, which hosted it until 2026-11-07, removed; reader-install until 2026-10-30)
#   make kindle-reader-log  the end of the reader's log on the Kindle (reader-log until 2026-10-30)
#   make kindle-status   awake or not, battery, Wi-Fi, free memory and storage
#   make kindle-wait     wait until it answers over SSH (after waking it by hand)
#   make kindle-ssh      a shell on it

KINDLE ?= kindle
SSH = ssh -o ConnectTimeout=10 $(KINDLE)

.PHONY: kindle-reader-install kindle-reader-log kindle-status kindle-wait kindle-ssh

kindle-reader-install: reader welcome
	@$(SSH) 'mkdir -p /mnt/us/shamsaink/sleep /mnt/us/shamsaink/books'
	@# a reader running from the launcher is stopped first (its launcher resumes the framework): its program, replaced on
	@# /mnt/us (FUSE) under it, would die of SIGBUS at its next page of code
	@$(SSH) 'for p in $$(pidof shamsaink); do kill $$p; echo "the open reader ($$p) stopped"; done; true'
	@# a new name, then renamed over the old: overwriting it in place fails now and then on /mnt/us (FUSE)
	@scp -q reader/out/shamsaink $(KINDLE):/mnt/us/shamsaink/shamsaink.new
	@$(SSH) 'mv /mnt/us/shamsaink/shamsaink.new /mnt/us/shamsaink/shamsaink'
	@# KOReader's plugin, which hosted the reader until the launcher (2026-11-07), and its pipes
	@$(SSH) 'rm -rf /mnt/us/koreader/plugins/shamsaink.koplugin /var/tmp/shamsaink'
	@# the launcher, and the scriptlet that lists it in the Kindle's library (reader/kindle)
	@scp -q reader/kindle/launch.sh $(KINDLE):/mnt/us/shamsaink/launch.sh
	@sh reader/kindle/scriptlet.sh | $(SSH) 'cat > /mnt/us/documents/ShamsaInk.sh'
	@# the fonts as the package has them: those no longer in it removed
	@$(SSH) 'mkdir -p /mnt/us/shamsaink/fonts && cd /mnt/us/shamsaink/fonts && for f in *.ttf; do case " $(notdir $(wildcard reader/fonts/*.ttf)) " in *" $$f "*) ;; *) rm -f "$$f";; esac; done'
	@scp -q reader/fonts/*.ttf $(KINDLE):/mnt/us/shamsaink/fonts/
	@# the getting-started guide beside the reader, as the package has it (copied into books/ on the first start)
	@scp -q reader/out/welcome/getting-started.epub $(KINDLE):/mnt/us/shamsaink/getting-started.epub
	@echo "reader, fonts, launcher and scriptlet copied to $(KINDLE)"

kindle-reader-log:
	@$(SSH) 'tail -n 40 /mnt/us/shamsaink/logs/reader.log'

kindle-status:
	@$(SSH) 'echo "kept awake: $$(lipc-get-prop com.lab126.powerd preventScreenSaver) (1 yes, by KOReader'"'"'s Keep alive or a script; 0 no)"; \
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
