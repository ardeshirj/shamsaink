# Helpers for the Kindle the experiments run on (docs/kindle-setup.md). KINDLE is the SSH host (~/.ssh/config).
#
#   make kindle-awake    keep it awake: no sleep, so Wi-Fi and SSH stay up (drains the battery: undo when done)
#   make kindle-sleep    let it sleep again (its own power manager, as normal)
#   make kindle-status   awake or not, battery, Wi-Fi, free memory and storage
#   make kindle-wait     wait until it answers over SSH (after waking it by hand)
#   make kindle-ssh      a shell on it

KINDLE ?= kindle
SSH = ssh -o ConnectTimeout=10 $(KINDLE)

.PHONY: kindle-awake kindle-sleep kindle-status kindle-wait kindle-ssh

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
