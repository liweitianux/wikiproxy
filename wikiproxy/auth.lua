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

local function exit(code, content)
    ngx.status = code
    ngx.header["Content-Type"] = "text/plain"
    ngx.header["Content-Length"] = #content + 1
    ngx.say(content)
end

function _M.handle()
    -- Simple authentication to prevent crawlers and public accesses/abuses.
    --
    local user_agent = ngx.var.http_user_agent
    if not user_agent or user_agent == "" then
        return exit(400, "bad request")
    end

    local client_ip = ngx.var.remote_addr
    local key_authed = "authed:" .. client_ip .. ":" .. user_agent
    if xshdict:get(key_authed) then
        return
    end

    local key_authing = "authing:" .. client_ip .. ":" .. user_agent
    local v = xshdict:incr(key_authing, 1, 0, xconfig.auth.wait_time)
    if v and v <= xconfig.auth.retries then
        local content = tostring(xconfig.auth.retries + 1 - v)
        return exit(xconfig.auth.code, content)
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
