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
local table = table

local ngx = ngx
local ngx_re = ngx.re
local ngx_req = ngx.req

local xconfig = require("wikiproxy.config")
local xhttp = require("wikiproxy.http")

------------------------------------------------------------------------
-- Preprocess the 'wikis' config.

local function escape_dot(d)
    d = ngx_re.gsub(d, [[\.]], [[\.]], "jo")
    return d
end

local wikis = {}
for _, w in ipairs(xconfig.wikis) do
    wikis[w.host] = w

    w._domains = {
        escape_dot(w.domain),
    }
    w._replacements = {
        [w.domain] = "",
    }
    w._prefixes = {}
    for _, m in ipairs(w.maps) do
        local domain, prefix = m[1], m[2]
        table.insert(w._domains, escape_dot(domain))
        w._replacements[domain] = prefix
        table.insert(w._prefixes, { domain, prefix, prefix .. "/" })
    end

    w._regex_groups = 3
    w._regex = table.concat({
        "(https?:)?",
        "//",
        "(" .. table.concat(w._domains, "|") .. ")",
        [[($|\s|[^a-zA-Z0-9_.])]],
    })
    ngx.log(ngx.DEBUG, "wiki [", w.domain, "] regex: ", w._regex)

    w._replace = function (scheme, hport)
        scheme = scheme .. ":"
        return function (m)
            local res = {
                "", -- scheme .. ":"
                "//",
                w.host,
                hport,
                w._replacements[m[2]],
                m[3],
            }
            if m[1] then -- Unmatched pattern has m[i] = false.
                res[1] = scheme
            end
            return table.concat(res)
        end
    end
end

------------------------------------------------------------------------

local function get_scheme_port()
end

-- Map the multiple domains used by Wikipedia to a unique path prefix.
local function map_urls(text, wiki, ctx)
    if text == nil or text == "" then
        return text
    end

    local replace = wiki._replace(ctx.scheme, ctx.hport)
    local newtext, _, err = ngx_re.gsub(text, wiki._regex, replace, "jo")
    if err then
        ngx.log(ngx.ERR, "ngx.re.gsub() failed: regex=", wiki._regex,
                ", error=", err)
        return text
    end

    return newtext
end

-- Recover the original domain and path.
local function unmap_path(path, wiki)
    for _, l in ipairs(wiki._prefixes) do
        local domain, prefix1, prefix2 = l[1], l[2], l[3]
        if path == prefix1 or path == prefix2 then
            return domain, "/"
        end
        local len = #prefix2
        if string.sub(path, 1, len) == prefix2 then
            path = string.sub(path, len)
            return domain, path
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
    -- Override the "Host", "Accept-Encoding"
    -- It seems OpenResty normalized the table keys to lowercase,
    -- but loop to locate the Host key just to be future-proof.
    for k, _ in pairs(headers) do
        local k_lc = string.lower(k)
        if k_lc == "host" then
            headers[k] = domain
        elseif k_lc == "accept-encoding" then
            headers[k] = nil -- XXX
            --headers[k] = "gzip" -- support gzip only
        end
    end

    local req = {
        scheme = "https",
        server = { domain, 443 },
        method = ngx_req.get_method(),
        path = path,
        query = ngx.var.args,
        headers = headers,
        body = body,
    }
    ngx.log(ngx.DEBUG, "request to wikipedia: domain=", domain,
            ", path=", req.path, ", query=", req.query)
    return req
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

    local ctx = {
        scheme = ngx.var.scheme,
        host = host,
        http_host = ngx.var.http_host,
        hport = "", -- optional port (format ":xxx") in the 'Host' header
    }
    do
        local m = ngx_re.match(ctx.http_host, [[(.*)(:\d+)$]], "jo")
        if m and m[2] then
            ctx.hport = m[2]
        end
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

    -- Transform the Location; for example, redirection to mobile site.
    local location = res.headers["Location"]
    if location then
        res.headers["Location"] = map_urls(location, wiki, ctx)
    end

    -- Transform the URLs in the page.
    local ct = res.headers["Content-Type"] or ""
    -- strip possible charset
    ct = ngx.re.gsub(ct, [[^\s*([\w/]+).*]], "$1", "jo")
    if ct == "text/html" or
       ct == "text/javascript" or
       ct == "text/css" -- e.g., url() in background attribute
    then
        -- TODO: support gzip'ed content (Content-Encoding: gzip)
        res.body = map_urls(res.body, wiki, ctx)
        res.headers["Content-Length"] = #res.body
    end

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
