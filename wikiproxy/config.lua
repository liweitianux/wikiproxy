-- SPDX-License-Identifier: MIT
--
-- WikiProxy Lua module
-- Configurations
--

local ngx = ngx

local _M = {
}


setmetatable(_M, {
    __newindex = function ()
        error("modification forbidden", 2)
    end,
})

return _M
