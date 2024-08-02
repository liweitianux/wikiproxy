-- SPDX-License-Identifier: MIT
--
-- WikiProxy Lua module
-- Proxy to Wikipedia and generate content.
--

local ngx = ngx

local _M = {}


function _M.handle()
    ngx.say("yoyo")
end


setmetatable(_M, {
    __newindex = function ()
        error("modification forbidden", 2)
    end,
})

return _M
