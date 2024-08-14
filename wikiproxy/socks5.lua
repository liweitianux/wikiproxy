-- SPDX-License-Identifier: MIT
--
-- WikiProxy Lua module
-- SOCKS5 proxy client-side routines.
--
-- Reference:
-- * https://datatracker.ietf.org/doc/html/rfc1928
-- * https://en.wikipedia.org/wiki/SOCKS#SOCKS5
--

local error = error
local math = math
local setmetatable = setmetatable
local string = string
local tonumber = tonumber
local tostring = tostring

local ngx = ngx

local xdns = require("wikiproxy.dns")
local xutil = require("wikiproxy.util")

-- NOTE: Lua 5.1 only supports '\ddd' (up to 3 *decimal* digits) in string,
--       while LuaJIT and Lua >=5.2 support '\xhh' syntax.
local _M = {
    -- socks version
    V5              = string.char(5),
    -- auth methods
    METHOD_NONE     = string.char(0),
    METHOD_GSSAPI   = string.char(0),
    -- commands
    CMD_TCP         = string.char(1),
    -- address type
    ATYPE_IPV4      = string.char(1),
    ATYPE_DOMAIN    = string.char(3),
    ATYPE_IPV6      = string.char(4),
    -- reply results
    REP_SUCCEEDED   = string.char(0),
    -- reserved zero byte
    RSV_ZERO        = string.char(0),
}


function _M.new(sock, url)
    local pattern = [[^(socks5h?)://(.+):(\d+)/?$]]
    local m, err = ngx.re.match(url, pattern, "jo")
    if not m or err then
        ngx.log(ngx.ERR, "failed to parse socks5 URL [", url, "]: ", err)
        return nil, err
    end

    local scheme, host, port = m[1], m[2], tonumber(m[3])

    -- NOTE: sock:connect() supports to resolve a domain name using the
    --       Nginx's core resolver configured with the 'resolver' option.
    --       However, we resolve it myself to simplify the config file.
    local addrs, err = xdns.resolve(host)
    if not addrs then
        ngx.log(ngx.ERR, "failed to resolve [", host, "]: ", err)
        return nil, err
    end
    local n = #addrs
    if n > 1 then
        n = math.random(n)
    end
    host = addrs[n]

    local obj = {
        sock = sock,
        scheme = scheme,
        host = host,
        port = port,
    }
    setmetatable(obj, {
        __index = function (table, key)
            local v = _M[key]
            if not v then
                v = sock[key]
            end
            return v
        end,
    })

    ngx.log(ngx.DEBUG, "created socks5 to: scheme=", scheme,
            ", host=", host, ", port=", port)
    return obj
end


function _M.is_socks5h(self)
    return self.scheme == "socks5h"
end


function _M.close(self)
    local ok, err = self.sock:setkeepalive()
    if not ok then
        ngx.log(ngx.ERR, "failed to set keepalive: ", err)
    end
    return ok, err
end


local function ichar(s, i)
    return string.char(string.byte(s, i))
end

local function send_greeting(sock)
    local data = {
        _M.V5,              -- version: SOCKS5
        string.char(2),     -- auth method count: 2
        _M.METHOD_NONE,     -- method 1: no auth
        _M.METHOD_GSSAPI,   -- method 2: GSSAPI (required by RFC)
    }
    local ok, err = sock:send(data)
    if not ok then
        ngx.log(ngx.ERR, "failed to send: ", err)
        return nil, err
    end
    ngx.log(ngx.DEBUG, "sent greeting")
    return true
end

local function read_choice(sock)
    -- Reply: Ver(1) + Method(1)
    local data, err = sock:receive(2)
    if not data then
        ngx.log(ngx.ERR, "failed to receive: ", err)
        return nil, err
    end
    if ichar(data, 1) ~= _M.V5 then
        err = "invalid server choice"
        ngx.log(ngx.ERR, err, ": ", tostring(data))
        return nil, err
    end
    -- We only support none auth.
    if ichar(data, 2) ~= _M.METHOD_NONE then
        err = "unsupported auth method"
        ngx.log(ngx.ERR, err, ": ", string.byte(data, 2))
        return nil, err
    end
    ngx.log(ngx.DEBUG, "received choice and is okay")
    return true
end

local function send_connection_request(sock, host, port, is_socks5h)
    if not is_socks5h then
        local addrs, err = xdns.resolve(host)
        if not addrs then
            ngx.log(ngx.ERR, "failed to resolve [", host, "]: ", err)
            return nil, err
        end
        local n = #addrs
        if n > 1 then
            n = math.random(n)
        end
        host = addrs[n]
    end

    local data = {
        _M.V5,          -- version: SOCKS5
        _M.CMD_TCP,     -- command: tcp connection
        _M.RSV_ZERO,    -- reserved
        "",             -- [4] address type
        "",             -- [5] address value
        xutil.htobe16(port), -- port (network byte order)
    }
    if xutil.is_ipv4(host) then
        data[4] = _M.ATYPE_IPV4
        data[5] = xutil.inet_addr(host)
    else
        local ok, ip6 = xutil.is_ipv6(host, true)
        if ok then
            data[4] = _M.ATYPE_IPV6
            data[5] = xutil.inet6_addr(ip6)
        else
            -- assume a domain name
            -- format: length(1) + domain(len)
            data[4] = _M.ATYPE_DOMAIN
            data[5] = string.char(#host) .. host
        end
    end

    local ok, err = sock:send(data)
    if not ok then
        ngx.log(ngx.ERR, "failed to send: ", err)
        return nil, err
    end
    ngx.log(ngx.DEBUG, "sent connection request")
    return true
end

local function read_connection_response(sock)
    -- Reply: Ver(1) + Rep(1) + Rsv(1) + AType(1) + BndAddr(...) + BndPort(2)
    local data, err = sock:receive(4) -- until AType
    if not data then
        ngx.log(ngx.ERR, "failed to receive: ", err)
        return nil, err
    end
    if ichar(data, 1) ~= _M.V5 then
        err = "invalid server response"
        ngx.log(ngx.ERR, err, ": ", tostring(data))
        return nil, err
    end
    if ichar(data, 2) ~= _M.REP_SUCCEEDED then
        err = "connection request failed"
        ngx.log(ngx.ERR, err, ": ", string.byte(data, 2))
        return nil, err
    end

    local rlen -- remaining length to read
    local atype = ichar(data, 4)
    if atype == _M.ATYPE_IPV4 then
        rlen = 4 + 2
    elseif atype == _M.ATYPE_IPV6 then
        rlen = 16 + 2
    elseif atype == _M.ATYPE_DOMAIN then
        -- read domain length
        local data, err = sock:receive(1)
        if not data then
            ngx.log(ngx.ERR, "failed to read domain length: ", err)
            return nil, err
        end
        rlen = string.byte(data) + 2
    else
        err = "unknown address type"
        ngx.log(ngx.ERR, err, ": ", string.byte(data, 4))
        return nil, err
    end
    -- read the remaining reply
    local data, err = sock:receive(rlen)
    if not data then
        ngx.log(ngx.ERR, "failed to read remaining reply: ", err)
        return nil, err
    end

    ngx.log(ngx.DEBUG, "connection accepted")
    return true
end

function _M.connect(self, host, port, options)
    local sock = self.sock
    local ok, err

    ok, err = sock:connect(self.host, self.port)
    if not ok then
        ngx.log(ngx.ERR, "failed to connect to proxy: host=", self.host,
                ", port=", self.port, ": ", err)
        return nil, err
    end

    ok, err = send_greeting(sock)
    if not ok then
        return err
    end
    ok, err = read_choice(sock)
    if not ok then
        return err
    end
    ok, err = send_connection_request(sock, host, port, self:is_socks5h())
    if not ok then
        return err
    end
    ok, err = read_connection_response(sock)
    if not ok then
        return err
    end

    ngx.log(ngx.DEBUG, "socks5 connection ready")
    return true
end


-- XXX: Although we've set the metatable to proxy the unknown methods
--      (e.g., this sslhandshake()) to the underlying 'self.sock', but
--      that didn't work.  Maybe the 'self' was wrong in that case...
--      Therefore, explicitly wrap the required methods for 'self.sock'.
function _M.sslhandshake(self, ...)
    ngx.log(ngx.DEBUG, "trying ssl handshake ...")
    return self.sock:sslhandshake(...)
end


setmetatable(_M, {
    __newindex = function ()
        error("modification forbidden", 2)
    end,
})

return _M
