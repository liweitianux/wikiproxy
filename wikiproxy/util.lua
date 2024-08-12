-- SPDX-License-Identifier: MIT
--
-- WikiProxy Lua module
-- Utilities
--

local string = string

local ngx_re = ngx.re

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
    if bracketed and string.sub(addr, 1, 1) == "[" then
        addr = string.sub(addr, 2, #addr - 1)
    end
    return ngx_re.find(addr, ipv6_regex, "jo") ~= nil
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


return _M
