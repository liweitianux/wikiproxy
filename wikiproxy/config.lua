-- SPDX-License-Identifier: MIT
--
-- WikiProxy Lua module
-- Configurations
--

local error = error
local setmetatable = setmetatable

local ngx = ngx

local _M = {
    -- Configure the Wikipedia sites and domain transformations.
    wikis = {
        -- English
        {
            -- domain to serve the proxy
            host = "en.wikiproxy.example.com",
            -- main domain of wikipedia
            domain = "en.wikipedia.org",
            -- maps of extra domains to path prefixes.
            -- (reuse the single domain to proxy the whole wikipedia site)
            maps = {
                -- [1] wikipedia's domain name
                -- [2] mapped path prefix (NOTE: start and end with '/')
                { "en.m.wikipedia.org",     "/.wp-m/" },
                { "www.wikimedia.org",      "/.wp-wm-www/" },
                { "upload.wikimedia.org",   "/.wp-wm-upload/" },
            },
        },
        -- Chinese
        {
            host = "zh.wikiproxy.example.com",
            domain = "zh.wikipedia.org",
            maps = {
                { "zh.m.wikipedia.org",     "/.wp-m/" },
                { "www.wikimedia.org",      "/.wp-wm-www/" },
                { "upload.wikimedia.org",   "/.wp-wm-upload/" },
            },
        },
    },

    -- Configure the simple authentication function.
    -- (implemented to protect from crawlers and public accesses/abuses)
    auth = {
        -- Status code to reply if not yet authenticated.
        code = ngx.HTTP_NOT_FOUND,
        -- Number of retries to pass the authentication.
        retries = 6,
        -- Time to wait for the client to finish the authentication.
        wait_time = 10, -- [seconds]
        -- Validity time of a successful authentication.
        ttl = 3600, -- [seconds]
    },

    -- DNS settings for resolving domain names.
    -- (unnecessary if use the socks5h variant proxy)
    dns = {
        nameservers = { "127.0.0.1", "[::1]" },
        timeout = 2, -- [seconds]
        retrans = 2, -- retransmissions on receive timeout
        prefer_ipv6 = false, -- whether prefer ipv4?
        cache = { -- LRU cache settings
            size = 256,
            ttl = 600, -- [seconds]
        },
    },

    -- SOCKS5 proxy to use.
    -- (also support the socks5h variant: let remote resolve the domain)
    proxy = "socks5h://127.0.0.1:1080",
}


setmetatable(_M, {
    __newindex = function ()
        error("modification forbidden", 2)
    end,
})

return _M
