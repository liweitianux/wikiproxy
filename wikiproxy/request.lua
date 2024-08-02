-- SPDX-License-Identifier: MIT
--
-- WikiProxy Lua module
-- Handle request and process access phase.
--

local ngx = ngx

local _M = {}


function _M.handle()
    -- TODO
end


setmetatable(_M, {
    __newindex = function ()
        error("modification forbidden", 2)
    end,
})

return _M
