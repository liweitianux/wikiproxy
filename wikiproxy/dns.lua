-- SPDX-License-Identifier: MIT
--
-- WikiProxy Lua module
-- DNS resolver
--

local error = error
local ipairs = ipairs
local string = string
local table = table

local ngx = ngx

local lrucache = require("resty.lrucache")
local resolver = require("resty.dns.resolver")

local xconfig = require("wikiproxy.config")
local xutil = require("wikiproxy.util")

------------------------------------------------------------------------
-- LRU cache

local cache_set, cache_get

do
    local conf = xconfig.dns.cache
    local _size = conf.size or 1024
    local _ttl = conf.ttl or 600 -- [seconds]
    local _cache

    local err
    _cache, err = lrucache.new(_size)
    if not _cache then
        error("failed to create lrucache: " .. err)
    end
    ngx.log(ngx.INFO, "created lrucache: size=", _size)

    function cache_set(key, value, ttl)
        ttl = ttl or _ttl
        _cache:set(key, value, ttl)
    end

    function cache_get(key)
        return _cache:get(key)
    end
end

------------------------------------------------------------------------

local _M = {}


-- NOTE: IPv6 address would be enclosed in brackets [], as required by
--       connect() and other routines.
function _M.resolve(name)
    if xutil.is_ipv4(name) then
        return name
    else
        local ok, ip6 = xutil.is_ipv6(name, true)
        if ok then
            return "[" .. ip6 .. "]"
        end
        -- else: assume a domain name.
    end

    name = string.lower(name)
    local addrs = cache_get(name)
    if addrs then
        ngx.log(ngx.DEBUG, "resolved [", name, "] from cache: ",
                table.concat(addrs, ", "))
        return addrs
    end

    -- NOTE: Do NOT store at module level; otherwise it would be shared by
    --       all the concurrent requests and lead to serious races.
    local r, err = resolver:new({
        nameservers = xconfig.dns.nameservers,
        timeout = xconfig.dns.timeout * 1000,
        retrans = xconfig.dns.retrans,
    })
    if not r then
        ngx.log(ngx.ERR, "failed to create resolver: ", err)
        return nil, err
    end

    local qtypes = { r.TYPE_A, r.TYPE_AAAA }
    if xconfig.dns.prefer_ipv6 then
        qtypes = { r.TYPE_AAAA, r.TYPE_A }
    end

    addrs = {}
    local n = 0
    for _, qtype in ipairs(qtypes) do
        local answers, err = r:query(name, { qtype = qtype })
        if not answers then
            ngx.log(ngx.ERR, "failed to query [", name, "] for [",
                    qtype, "] records: ", err)
            return nil, err
        end
        if answers.errcode then
            ngx.log(ngx.WARN, "query [", name, "] for [", qtype,
                    "] records returned error: ",
                    answers.errcode, ", ", answers.errstr)
        else
            for _, ans in ipairs(answers) do
                ngx.log(ngx.DEBUG, "resolved: ", ans.name,
                        ", type: ", ans.type,
                        ", result: ", ans.address or ans.cname)
                if ans.type == qtype then
                    n = n + 1
                    if qtype == r.TYPE_AAAA then
                        addrs[n] = "[" .. ans.address .. "]"
                    else
                        addrs[n] = ans.address
                    end
                end
            end
        end
        if n > 0 then
            break
        end
    end

    if n == 0 then
        return nil, "no address resolved"
    end

    cache_set(name, addrs)
    ngx.log(ngx.DEBUG, "resolved [", name, "] to addresses: ",
            table.concat(addrs, ", "))
    return addrs
end


setmetatable(_M, {
    __newindex = function ()
        error("modification forbidden", 2)
    end,
})

return _M
