-- vSFG-7 SkyEye ATC: tell SkyEye which mission the DCS server has loaded.
--
-- Every time a mission loads (including each step of a weather rotation) this
-- writes the mission's full path to OUT. Every atc.exe role polls that file
-- (--miz-watch-file, default below) and applies the new mission's weather live,
-- so towers, ATIS, Command and the carrier roles stop announcing the first
-- mission's wind/ceiling/altimeter after a rotation. The launcher's .miz weather
-- widget reads the same file.
--
-- INSTALL on each rig that runs the DCS server (Foothold .222, Training 1 .220):
--   copy this file to  <server Saved Games>\Scripts\Hooks\
--   e.g. C:\Users\Administrator\Saved Games\DCS.dcs_serverrelease\Scripts\Hooks\
--   then restart the DCS server once. Nothing to configure in SkyEye if SkyEye
--   lives in C:\SkyeyeATC; otherwise edit OUT and pass --miz-watch-file.
--
-- Check it works: after a mission loads, C:\SkyeyeATC\current_mission.txt holds the
-- .miz path, dcs.log has a "VSFG7-MIZ current mission:" line, and each SkyEye role
-- logs "Weather reloaded — DCS server loaded a different mission".

local OUT = [[C:\SkyeyeATC\current_mission.txt]]

local cb = {}

local function writeCurrentMission()
    local ok, err = pcall(function()
        local path = DCS.getMissionFilename()
        if path == nil or path == "" then
            return
        end
        -- Write to a temp file and rename, so SkyEye never reads half a path.
        local tmp = OUT .. ".tmp"
        local f = io.open(tmp, "w")
        if not f then
            log.write("VSFG7-MIZ", log.ERROR, "cannot write " .. tmp)
            return
        end
        f:write(path)
        f:close()
        os.remove(OUT)
        os.rename(tmp, OUT)
        log.write("VSFG7-MIZ", log.INFO, "current mission: " .. path)
    end)
    if not ok then
        log.write("VSFG7-MIZ", log.ERROR, tostring(err))
    end
end

cb.onMissionLoadEnd = writeCurrentMission
cb.onSimulationStart = writeCurrentMission

DCS.setUserCallbacks(cb)
log.write("VSFG7-MIZ", log.INFO, "hook loaded, writing the current mission to " .. OUT)
