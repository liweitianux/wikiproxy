-- SPDX-License-Identifier: MIT
--
-- WikiProxy Lua module
-- Utilities
--

local assert = assert
local error = error
local setmetatable = setmetatable
local string = string

local ngx_re = ngx.re

local bit = require("bit") -- require LuaJIT

-- Credit: https://stackoverflow.com/a/17871737
-- See also: https://stackoverflow.com/a/36760050
local ipv4_regex = [[^((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])$]]
local ipv6_regex = [[^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$]]


local _M = {}


function _M.is_ipv4(addr)
    if not addr or addr == "" then
        return false
    end
    return ngx_re.find(addr, ipv4_regex, "jo") ~= nil
end


-- Check whether <addr> is an IPv6 address?
-- If <bracketed> is true, then also supports a bracket-enclosed IPv6
-- address, like one appearing in URL.
function _M.is_ipv6(addr, bracketed)
    if not addr or addr == "" then
        return false
    end
    if not string.find(addr, ":", 1, true) then
        return false
    end
    if bracketed and string.byte(addr, 1) == string.byte("[") then
        addr = string.sub(addr, 2, #addr - 1)
    end
    if ngx_re.find(addr, ipv6_regex, "jo") then
        return true, addr
    else
        return false
    end
end


--[=[
-- https://gist.github.com/syzdek/6086792
local addrs = [[
127.0.0.1
10.0.0.1
192.168.1.1
0.0.0.0
255.255.255.255

10002.3.4
1.2.3.4.5
256.0.0.0
260.0.0.0
]]

for addr in string.gmatch(addrs, "[^\n]+") do
    print(addr, _M.is_ipv6(addr))
end

local addrs = [[
1:2:3:4:5:6:7:8
::ffff:10.0.0.1
::ffff:1.2.3.4
::ffff:0.0.0.0
1:2:3:4:5:6:77:88
::ffff:255.255.255.255
fe08::7:8
ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff

1:2:3:4:5:6:7:8:9
1:2:3:4:5:6::7:8
:1:2:3:4:5:6:7:8
1:2:3:4:5:6:7:8:
::1:2:3:4:5:6:7:8
1:2:3:4:5:6:7:8::
1:2:3:4:5:6:7:88888
2001:db8:3:4:5::192.0.2.33
fe08::7:8%
fe08::7:8i
fe08::7:8interface
]]

for addr in string.gmatch(addrs, "[^\n]+") do
    print(addr, _M.is_ipv6(addr))
end
--]=]


function _M.htobe16(n)
    assert(n >= 0 and n <= 65535)
    return string.char(bit.band(0xFF, bit.rshift(n, 8)), bit.band(0xFF, n))
end

------------------------------------------------------------------------

local ffi = require("ffi")
local C = ffi.C

ffi.cdef[[
typedef uint32_t        in_addr_t;
typedef unsigned char   u_char;
typedef intptr_t        ngx_int_t; /* nginx/src/core/ngx_config.h */

/* nginx/src/core/ngx_inet.h */
in_addr_t ngx_inet_addr(u_char *text, size_t len);
ngx_int_t ngx_inet6_addr(u_char *p, size_t len, u_char *addr);
]]

local inaddr_none = ffi.cast("in_addr_t", 0xFFFFFFFF)
local inet6_ibufsize = 64
local inet6_ibuffer = ffi.new(ffi.typeof("u_char[?]"), inet6_ibufsize)
local inet6_obuffer = ffi.new(ffi.typeof("u_char[?]"), 16)

-- Convert IPv4 address from text format to binary format.
function _M.inet_addr(ip4)
    -- The Nginx function is declared with type 'u_char *' rather than
    -- 'const u_char *', so FFI prevents from passing Lua string to it.
    local len = #ip4
    if len > inet6_ibufsize then
        return nil, "invalid ipv4 address"
    end
    local ibuf = inet6_ibuffer
    ffi.copy(ibuf, ip4, len)

    local addr = C.ngx_inet_addr(ibuf, len)
    if addr == inaddr_none then
        return nil, "invalid ipv4 address"
    end

    local addrp = ffi.new("in_addr_t[1]", addr)
    local obuf = inet6_obuffer
    ffi.copy(obuf, addrp, 4)
    return ffi.string(obuf, 4)
end

-- Convert IPv6 address from text format to binary format.
function _M.inet6_addr(ip6)
    local len = #ip6
    if len > inet6_ibufsize then
        return nil, "invalid ipv6 address"
    end
    local ibuf = inet6_ibuffer
    ffi.copy(ibuf, ip6, len)

    local obuf = inet6_obuffer
    local rc = C.ngx_inet6_addr(ibuf, len, obuf)
    if rc ~= 0 then
        return nil, "invalid ipv6 address"
    end
    return ffi.string(obuf, 16)
end

do
    local addr

    addr = _M.inet_addr("111111")
    assert(addr == nil)

    addr = _M.inet_addr("127.0.0.1")
    assert(addr == "\127\0\0\1")
    ngx.log(ngx.INFO, "inet_addr() test passed")

    addr = _M.inet6_addr("::1")
    assert(addr == "\0\0\0\0".."\0\0\0\0".."\0\0\0\0".."\0\0\0\1")
    ngx.log(ngx.INFO, "inet6_addr() test passed")
end

------------------------------------------------------------------------

setmetatable(_M, {
    __newindex = function ()
        error("modification forbidden", 2)
    end,
})

return _M
