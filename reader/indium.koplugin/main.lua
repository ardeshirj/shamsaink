--[[--
Indium from KOReader: "Open with Indium" on an EPUB's long-press dialog (file manager, History, Collections, search).
KOReader lets go of its input devices (it grabs them, EVIOCGRAB), runs the reader on the book, then takes them back
the way it opened them at start and redraws its screen. The reader, its fallback fonts and its log are in
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

-- open runs the reader on a book, once the dialog has closed and the screen is redrawn without it.
local function open(file)
    UIManager:scheduleIn(0.5, function()
        run("cd " .. dir .. " && { echo \"== $(date -u '+%Y-%m-%d %H:%M UTC')\"; ./indium "
            .. util.shell_escape({ file }) .. " fonts; } >> log.txt 2>&1")
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
