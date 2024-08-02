-- SPDX-License-Identifier: MIT
--
-- WikiProxy Lua module
-- Configurations
--

local ngx = ngx

local _M = {
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
}


setmetatable(_M, {
    __newindex = function ()
        error("modification forbidden", 2)
    end,
})

return _M
