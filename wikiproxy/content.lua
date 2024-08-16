-- SPDX-License-Identifier: MIT
--
-- WikiProxy Lua module
-- Proxy to Wikipedia and generate content.
--

local error = error
local io = io
local ipairs = ipairs
local pairs = pairs
local require = require
local setmetatable = setmetatable
local string = string

local ngx = ngx
local ngx_req = ngx.req

local xconfig = require("wikiproxy.config")
local xhttp = require("wikiproxy.http")

------------------------------------------------------------------------
-- Preprocess the 'wikis' config.
local wikis = {}
for _, w in ipairs(xconfig.wikis) do
    wikis[w.host] = w
    -- TODO: more for re.gsub()
end

------------------------------------------------------------------------

-- Recover the original domain and path.
local function unmap_path(path, wiki)
    for _, m in ipairs(wiki.maps) do
        local d_wiki, prefix = m[1], m[2]
        local len = #prefix
        if string.sub(path, 1, len) == prefix then
            path = string.sub(path, len)
            return d_wiki, path
        end
    end

    return wiki.domain, path
end

-- Read the content of request body, which may be either in a memory buffer
-- or in a temporary file.
local function get_body()
    ngx_req.read_body()

    local body = ngx_req.get_body_data()
    if body then
        return body
    end

    local path = ngx_req.get_body_file()
    if not path then
        return nil, nil -- no request body
    end

    ngx.log(ngx.DEBUG, "attempt to read body from file: ", path)
    local f, err = io.open(path, "rb")
    if not f then
        ngx.log(ngx.ERR, "failed to open file [", path, "]: ", err)
        return nil, err
    end
    body = f:read("*all")
    f:close()
    return body
end

local function make_request(wiki)
    local body, err = get_body() -- body may be nil
    if err then
        return nil, err
    end

    local path = ngx.var.uri
    local domain, path, err = unmap_path(path, wiki)
    if err then
        return nil, err
    end

    local headers = ngx_req.get_headers()
    -- Override the "Host".
    -- It seems OpenResty normalized the table keys to lowercase,
    -- but loop to locate the Host key just to be future-proof.
    for k, _ in pairs(headers) do
        if string.lower(k) == "host" then
            headers[k] = domain
        end
    end

    return {
        scheme = "https",
        server = { domain, 443 },
        method = ngx_req.get_method(),
        path = path,
        query = ngx.var.args,
        headers = headers,
        body = body,
    }
end

------------------------------------------------------------------------

local _M = {}

function _M.handle()
    local host = ngx.var.host
    local wiki = wikis[host]
    if not wiki then
        ngx.status = ngx.HTTP_NOT_FOUND
        return ngx.say("404 not found")
    end

    local req, err = make_request(wiki)
    if not req then
        ngx.status = ngx.HTTP_BAD_REQUEST
        return ngx.say("400 bad request: cannot make request")
    end

    -- Request to Wikipedia.
    local res, err = xhttp.request(req, { proxy = xconfig.proxy })
    if not res then
        ngx.status = ngx.HTTP_BAD_REQUEST
        return ngx.say("400 bad request: cannot proxy request")
    end

    -- Adjust headers.
    res.headers["Connection"] = nil -- Nginx will auto set this.
    res.headers["Trailer"] = nil

    -- TODO: location substitution (-> mobile redirection)
    -- TODO: content substitution (support gzip decomp)

    -- Send response.
    ngx.status = res.status
    for k, v in pairs(res.headers) do
        ngx.header[k] = v
    end
    if res.trailers then
        for k, v in pairs(res.trailers) do
            ngx.header[k] = v
        end
    end
    if res.body then
        ngx.print(res.body)
    end
end


setmetatable(_M, {
    __newindex = function ()
        error("modification forbidden", 2)
    end,
})

return _M
