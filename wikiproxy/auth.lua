-- SPDX-License-Identifier: MIT
--
-- WikiProxy Lua module
-- Handle request and process access phase.
--

local error = error
local require = require
local setmetatable = setmetatable
local tostring = tostring

local ngx = ngx

local xconfig = require("wikiproxy.config")

local xshdict = ngx.shared["wikiproxy"]

local _M = {}


function _M.handle()
    -- Simple authentication to prevent crawlers and public accesses/abuses.
    --
    local client_ip = ngx.var.remote_addr
    local user_agent = ngx.var.http_user_agent

    local key_authed = "authed:" .. client_ip .. ":" .. user_agent
    if xshdict:get(key_authed) then
        return
    end

    local key_authing = "authing:" .. client_ip .. ":" .. user_agent
    local v = xshdict:incr(key_authing, 1, 0, xconfig.auth.wait_time)
    if v and v <= xconfig.auth.retries then
        ngx.status = xconfig.auth.code
        ngx.say(tostring(xconfig.auth.retries + 1 - v))
        return
    end

    xshdict:set(key_authed, "dummy", xconfig.auth.ttl)
    ngx.log(ngx.INFO, "authenticated: ip=[", client_ip, "], user_agent=[",
            user_agent, "]")
end


setmetatable(_M, {
    __newindex = function ()
        error("modification forbidden", 2)
    end,
})

return _M
