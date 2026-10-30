-- KOReader user patch (copy to koreader/patches/ on the Kobo): on a MediaTek Kobo, Exit becomes Reboot while Kobo's
-- Bluetooth daemon runs. Exit restarts Nickel (koreader.sh runs nickel.sh), and once Bluetooth has been on, Nickel's
-- second load of MediaTek's Wi-Fi/Bluetooth driver panics the kernel: the device reboots and NickelMenu is lost
-- (koreader/koreader#12739; experiments/13-kobo/NOTES.md). A reboot never starts Nickel a second time.
-- Only a plain exit (code 0) is changed; Restart (85), USB mode (86) and Reboot or Power off (88) go through as they are.
local Device = require("device")
local UIManager = require("ui/uimanager")
local logger = require("logger")

if not (Device:isKobo() and Device:isMTK()) then
    return
end

local function bluetoothDaemonRuns()
    -- not running until Bluetooth is first switched on in a boot, running from then on
    return os.execute("pidof mtkbtd >/dev/null 2>&1") == 0
end

local quit = UIManager.quit
UIManager.quit = function(self, exit_code, implicit)
    if (exit_code == nil or exit_code == 0) and (self._exit_code == nil or self._exit_code == 0)
            and bluetoothDaemonRuns() then
        logger.info("kobo-mtk-bt-exit: Bluetooth's daemon runs, so Exit reboots instead of restarting Nickel")
        -- 88 makes koreader.sh leave without restarting Nickel; the reboot comes once KOReader has saved and exited
        os.execute("sleep 3 && reboot &")
        return quit(self, 88, implicit)
    end
    return quit(self, exit_code, implicit)
end
