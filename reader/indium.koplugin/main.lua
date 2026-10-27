--[[--
Indium from KOReader: "Open with Indium" on an EPUB's long-press dialog (file manager, History, Collections, search).
KOReader lets go of its input devices (it grabs them, EVIOCGRAB), hands the book to the reader (started the first
time, then kept running) and waits until it is left, then takes the devices back the way it opened them at start and
redraws its screen. The reader, its fallback fonts and its log are in
/mnt/us/indium.
--]]--

local Device = require("device")
local FileManager = require("apps/filemanager/filemanager")
local UIManager = require("ui/uimanager")
local WidgetContainer = require("ui/widget/container/widgetcontainer")
local logger = require("logger")
local util = require("util")

local dir = "/mnt/us/indium"

local Indium = WidgetContainer:extend{
    name = "indium",
    is_doc_only = false,
}

-- run releases KOReader's input devices, runs cmd (blocking KOReader meanwhile), and takes them back.
local function run(cmd)
    local function night()
        return string.format("hw night %s, setting %s", tostring(Device.screen:getHWNightmode()),
            tostring(G_reader_settings:isTrue("night_mode")))
    end
    local paths = {}
    for path in pairs(Device.input.opened_devices) do
        if path:match("^/dev/input/") then
            table.insert(paths, path)
        end
    end
    for _, path in ipairs(paths) do
        Device.input:close(path)
    end
    logger.info("indium: released", table.concat(paths, " "), "; running", cmd, ";", night())
    local status = os.execute(cmd)
    -- As KOReader opened them at start (a scan, the devices kept by fd): Input:open(path) alone leaves a device
    -- that the next close does not let go of.
    if Device.openInputDevices then
        Device:openInputDevices()
    else
        for _, path in ipairs(paths) do
            Device.input:open(path)
        end
    end
    logger.info("indium: exit", status, "; took back", table.concat(paths, " "), ";", night())
    UIManager:setDirty("all", "full")
end

-- The reader is kept running (it starts once: its start is slow from /mnt/us) and handed each book through two pipes
-- in RAM: the book's path written to `in`, then a line read from `out` when the book is left, or its end if the
-- reader died meanwhile.
local pipes = "/var/tmp/indium"

-- open runs the reader on a book, once the dialog has closed and the screen is redrawn without it.
local function open(file)
    local log = dir .. "/log.txt"
    local cmd = table.concat({
        "P=" .. pipes .. ";",
        -- started unless running; its pid is written once its fonts are loaded, waited for up to 10 s
        "alive() { pid=$(cat $P/pid 2>/dev/null); [ -n \"$pid\" ] && [ -d /proc/$pid ]; };",
        "if ! alive; then",
        "  rm -rf $P && mkdir -p $P && mkfifo $P/in $P/out &&",
        "  echo \"== $(date -u '+%Y-%m-%d %H:%M UTC') started\" >> " .. log .. " &&",
        "  (cd " .. dir .. " && exec ./indium -serve $P fonts >> " .. log .. " 2>&1 &);",
        "  i=0; while [ ! -s $P/pid ] && [ $i -lt 100 ]; do usleep 100000; i=$((i+1)); done;",
        "fi;",
        -- written only to a reader alive, else the write would wait for one forever
        "alive &&",
        "echo \"== $(date -u '+%Y-%m-%d %H:%M UTC')\" >> " .. log .. " &&",
        "printf '%s\\n' " .. util.shell_escape({ file }) .. " > $P/in &&",
        "head -n 1 < $P/out",
    }, " ")
    UIManager:scheduleIn(0.5, function()
        run(cmd)
    end)
end

FileManager.addFileDialogButtons(FileManager, "indium", function(file, is_file)
    if is_file and file:lower():match("%.epub$") then
        return {
            {
                text = "Open with Indium",
                callback = function()
                    local chooser = FileManager.instance and FileManager.getMenuInstance()
                    if chooser and chooser.file_dialog then
                        UIManager:close(chooser.file_dialog)
                    end
                    open(file)
                end,
            },
        }
    end
end)

return Indium
