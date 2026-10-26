--[[--
Experiment 11, step 1: KOReader lets go of its input devices (it grabs them, EVIOCGRAB), runs one of our programs,
then opens them again and redraws its screen. "Indium: touch test" runs input-probe for 20 s with the touch grabbed.
--]]--

local Device = require("device")
local UIManager = require("ui/uimanager")
local WidgetContainer = require("ui/widget/container/widgetcontainer")
local logger = require("logger")

local dir = "/mnt/us/indium-09"

local Indium = WidgetContainer:extend{
    name = "indium",
    is_doc_only = false,
}

function Indium:init()
    self.ui.menu:registerToMainMenu(self)
end

-- run releases KOReader's input devices, runs cmd (blocking KOReader meanwhile), and takes them back.
local function run(cmd)
    local paths = {}
    for path in pairs(Device.input.opened_devices) do
        if path:match("^/dev/input/") then
            table.insert(paths, path)
        end
    end
    for _, path in ipairs(paths) do
        Device.input:close(path)
    end
    local function night()
        return string.format("hw night %s, setting %s", tostring(Device.screen:getHWNightmode()),
            tostring(G_reader_settings:isTrue("night_mode")))
    end
    logger.info("indium:", night())
    logger.info("indium: released", table.concat(paths, " "), "; running", cmd)
    local status = os.execute(cmd)
    for _, path in ipairs(paths) do
        Device.input:open(path)
    end
    logger.info("indium: exit", status, "; took back", table.concat(paths, " "))
    logger.info("indium:", night())
    UIManager:setDirty("all", "full")
end

function Indium:addToMainMenu(menu_items)
    menu_items.indium_touch_test = {
        text = "Indium: touch test",
        sorting_hint = "more_tools",
        callback = function()
            -- After the menu has closed and the screen is redrawn without it.
            UIManager:scheduleIn(0.5, function()
                run("cd " .. dir .. " && { echo \"== $(date -u '+%Y-%m-%d %H:%M UTC') touch test\"; "
                    .. "bin/input-probe taps 20 grab; } >> results-input.txt 2>&1")
            end)
        end,
    }
end

return Indium
