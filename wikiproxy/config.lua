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
        -- domain to serve the proxy -> configs
        ["en.example.com"] = {
            -- List of domain transformations.
            -- (reuse the single domain to proxy the whole site)
            domains = {
                -- [1] wikipedia's domain name
                -- [2] transformed path (NOTE: start and end with '/')
                { "en.wikipedia.org",       "/.wp/" },
                { "en.m.wikipedia.org",     "/.wp-m/" },
                { "www.wikimedia.org",      "/.wp-wm-www/" },
                { "upload.wikimedia.org",   "/.wp-wm-upload/" },
            },
        },
        -- Chinese
        ["zh.example.com"] = {
            domains = {
                { "zh.wikipedia.org",       "/.wp/" },
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
    -- This is unnecessary if the socks5h variant is used.
    dns = {
        nameservers = { "127.0.0.1" },
        timeout = 2, -- [seconds]
        retrans = 2, -- retransmissions on receive timeout
        prefer_ipv6 = false, -- whether prefer ipv4?
        cache = { -- LRU cache settings
            size = 256,
            ttl = 600, -- [seconds]
        },
    },
}


setmetatable(_M, {
    __newindex = function ()
        error("modification forbidden", 2)
    end,
})

return _M
