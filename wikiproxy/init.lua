-- SPDX-License-Identifier: MIT
--
-- WikiProxy Lua module
--

local ngx = ngx

local _M = {}


function _M.init()
    ngx.log(ngx.DEBUG, "prefix: ", ngx.config.prefix())
    ngx.log(ngx.DEBUG, "package.path: ", package.path)
end


function _M.init_worker()
    -- nothing
end


function _M.access_phase()
    require("wikiproxy.auth").handle()
end


function _M.content_phase()
    require("wikiproxy.content").handle()
end


setmetatable(_M, {
    __newindex = function ()
        error("modification forbidden", 2)
    end,
})

return _M
