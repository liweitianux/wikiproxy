-- SPDX-License-Identifier: MIT
--
-- WikiProxy Lua module
-- Simple HTTP request routines
--
-- Credit: https://github.com/ledgetech/lua-resty-http
--

local error = error
local ipairs = ipairs
local math = math
local pairs = pairs
local rawget = rawget
local rawset = rawset
local setmetatable = setmetatable
local string = string
local table = table
local tonumber = tonumber
local tostring = tostring
local type = type

local ngx = ngx

local xdns = require("wikiproxy.dns")

------------------------------------------------------------------------

local Headers = {}

Headers.__newindex = function ()
    error("modification forbidden", 2)
end

function Headers.new(headers)
    local mt = {
        normalized = {},
    }

    mt.__index = function (t, k)
        return rawget(t, mt.normalized[string.lower(k)])
    end

    mt.__newindex = function (t, k, v)
        local k_lower = string.lower(k)
        local k_orig = mt.normalized[k_lower]
        if not k_orig then
            -- First time seeing this header field.
            -- Creat a lowercased entry in the metadata proxy, with the
            -- value of the given field case.
            mt.normalized[k_lower] = k
            -- Set the header using the given field case.
            rawset(t, k, v)
        else
            -- Use the metadata proxy to get the original field case,
            -- and update its value.
            rawset(t, k_orig, v)
        end
    end

    local h = {}
    setmetatable(h, mt)

    if type(headers) == "table" then
        for k, v in pairs(headers) do
            h[k] = v
        end
    end

    return h
end

------------------------------------------------------------------------

local Client = {
    _USER_AGENT = "WikiProxy/1.0",
}

-- Reuse the class table to store its metamethods.
Client.__index = Client
-- Make the instantiated object (table) read-only.
Client.__newindex = function ()
    error("modification forbidden", 2)
end


function Client.new()
    local sock, err = ngx.socket.tcp()
    if not sock then
        ngx.log(ngx.ERR, "socket.tcp() failed: ", err)
        return nil, err
    end

    local obj = {
        sock = sock,
        keepalive = true,
    }
    setmetatable(obj, Client)
    return obj
end


function Client.close(self)
    return self.sock:close()
end


function Client.set_keepalive(self)
    local sock = self.sock
    local ok, err

    if self.keepalive then
        ok, err = sock:setkeepalive()
        if not ok then
            ngx.log(ngx.ERR, "failed to set keepalive: ", err)
        end
    else
        ok, err = sock:close()
        if not ok then
            ngx.log(ngx.ERR, "failed to close: ", err)
        end
    end

    return ok, err
end


-- TODO: support socks5 proxy ...
function Client.connect(self, host, port, options)
    local sock = self.sock

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

    -- NOTE: sock:connect() supports to resolve a domain name using the
    --       Nginx's core resolver configured with the 'resolver' option.
    --       However, we perform the resolution above for fine control and
    --       better maintainbility.
    -- NOTE: IPv6 address must be enclosed in [] for connect().
    local ok, err = sock:connect(host, port, { pool = options.pool_name })
    if not ok then
        ngx.log(ngx.ERR, "failed to connect: host=", host,
                ", port=", port, ": ", err)
        return nil, err
    end

    if options.ssl then
        local sni = options.ssl_server_name
        ok, err = sock:sslhandshake(nil, sni, options.ssl_verify)
        if not ok then
            ngx.log(ngx.ERR, "SSL handshake failed: host=", host,
                    ", port=", port, ", sni=", sni, ": ", err)
            return nil, err
        end
    end

    ngx.log(ngx.DEBUG, "connected to: host=", host,
            ", port=", port, ", ssl=", options.ssl)
    return true, nil
end


local function is_chunked(headers)
    local te = headers["Transfer-Encoding"]
    if not te then
        return false
    end
    return string.lower(te) == "chunked"
end


-- NOTE: The <req> table will be modified in-place.
-- NOTE: Only supports HTTP/1.1 for simplicity.
function Client.send_request(self, req)
    local sock = self.sock
    local method = string.upper(req.method)
    local headers = req.headers
    local body = req.body

    -- Prepare the headers.
    if is_chunked(headers) then
        -- Drop 'Content-Length' for chunked transfer, to help prevent
        -- request smuggling.
        headers["Content-Length"] = nil
    elseif not headers["Content-Length"] then
        -- Try to calculate the Content-Length.
        local want_body = (method == "POST" or
                           method == "PUT" or
                           method == "PATCH")
        if body == nil and want_body then
            headers["Content-Length"] = 0
        elseif type(body) == "table" then
            local len = 0
            for _, v in ipairs(body) do
                len = len + #tostring(v)
            end
            headers["Content-Length"] = len
        elseif body ~= nil then
            headers["Content-Length"] = #tostring(body)
        end
    end
    if not headers["User-Agent"] then
        headers["User-Agent"] = Client._USER_AGENT
    end

    -- Format the request header.
    local query = req.query or ""
    if type(query) == "table" then
        query = ngx.encode_args(query)
    end
    local hdr = {
        string.upper(method),
        " ",
        req.path,
        query == "" and "" or "?",
        query,
        " ",
        "HTTP/1.1\r\n",
    }
    local n = #hdr + 1
    for key, values in pairs(headers) do
        key = tostring(key)
        if type(values) == "table" then
            for _, v in ipairs(values) do
                hdr[n] = key .. ": " .. tostring(v) .. "\r\n"
                n = n + 1
            end
        else
            hdr[n] = key .. ": " .. tostring(values) .. "\r\n"
            n = n + 1
        end
    end
    hdr[n] = "\r\n"
    --ngx.log(ngx.DEBUG, "request header:\n", table.concat(hdr))

    -- Send the request header.
    local bytes, err = sock:send(hdr)
    if not bytes then
        ngx.log(ngx.ERR, "failed to send request header: ", err)
        return nil, err
    end

    -- Send the request body.
    -- NOTE: Don't support 100-Continue/Expect for simplicity.
    if body then
        bytes, err = sock:send(body)
        if not bytes then
            ngx.log(ngx.ERR, "failed to send request body: ", err)
            return nil, err
        end
    end

    return true
end


local function receive_headers(sock)
    local headers = Headers.new()
    local line, err

    while true do
        line, err = sock:receive("*l")
        if not line then
            ngx.log(ngx.ERR, "failed to receive header: ", err)
            return nil, err
        end

        if line == "" or ngx.re.find(line, [[^\s*$]], "jo") then
            break
        end

        local m, err = ngx.re.match(line, [[([^:\s]+):\s*(.*)]], "jo")
        if not m or err then
            ngx.log(ngx.ERR, "failed to parse header line [", line,
                    "], error=", err)
            goto continue
        end

        local key, val = m[1], m[2]
        local val_old = headers[key]
        if val_old then
            if type(val_old) == "table" then
                table.insert(val_old, tostring(val))
            else
                headers[key] = { val_old, tostring(val) }
            end
        else
            headers[key] = tostring(val)
        end

        ::continue::
    end

    return headers
end


local function should_receive_body(method, status)
    if method == "HEAD" then
        return false
    end
    if status == 204 or status == 304 then
        return false
    end
    if status >= 100 and status < 200 then
        return false
    end
    return true
end


local function read_body_chunked(sock)
    local remaining = 0
    local chunks = {}
    local n = 1
    local length, data, err

    repeat
        -- Receive the chunk size.
        data, err = sock:receive("*l")
        if not data then
            ngx.log(ngx.ERR, "failed to receive chunk size: ", err)
            return nil, err
        end
        length = tonumber(data, 16) -- size in hexdecimal
        if not length then
            ngx.log(ngx.ERR, "invalid chunk size: ", data)
            return nil, "invalid chunk size"
        end

        if length > 0 then
            -- Receive the chunk data.
            data, err = sock:receive(length)
            if not data then
                ngx.log(ngx.ERR, "failed to receive chunk data: ", err)
                return nil, err
            end
            chunks[n] = data
            n = n + 1
        end

        -- Consume the CR+LF.
        sock:receive(2)
    until length == 0

    return table.concat(chunks)
end


function Client.read_response(self, req)
    local sock = self.sock

    -- Receive the status line.
    local line, err = sock:receive("*l")
    if not line then
        ngx.log(ngx.ERR, "failed to receive status line: ", err)
        return nil, err
    end

    -- e.g., HTTP/1.1 200 OK
    local version = tonumber(string.sub(line, 6, 8))
    if not version then
        ngx.log(ngx.ERR, "failed to get HTTP version from: ", line)
        return nil, "invalid status line"
    end
    local status = tonumber(string.sub(line, 10, 12))
    if not version then
        ngx.log(ngx.ERR, "failed to get status code from: ", line)
        return nil, "invalid status line"
    end
    local reason = string.sub(line, 14)

    -- Receive headers.
    local headers, err = receive_headers(sock)
    if not headers then
        return nil, err
    end

    -- Determine the keepalive response.
    local h_connection = headers["Connection"]
    if version == 1.1 and h_connection and
       string.find(string.lower(h_connection), "close", 1, true)
    then
        self.keepalive = false
    end

    -- Receive the body.
    local body
    if should_receive_body(req.method, status) then
        if version == 1.1 and is_chunked(headers) then
            body, err = read_body_chunked(sock)
        else
            local length = tonumber(headers["Content-Length"])
            if length then
                body, err = sock:receive(length)
            else
                -- No length, so read everything.
                body, err = sock:receive("*a")
            end
        end
        if not body then
            ngx.log(ngx.ERR, "failed to read body: ", err)
            return nil, err
        end
    end

    -- Receive the trailer if present.
    local trailers
    if headers["Trailer"] then
        trailers, err = receive_headers(sock)
        if not trailers then
            ngx.log(ngx.ERR, "failed to receive trailers: ", err)
            return nil, err
        end
        setmetatable(headers, { __index = trailers })
    end

    return {
        version = version,
        status = status,
        reason = reason,
        headers = headers,
        trailers = trailers,
        body = body,
    }
end

------------------------------------------------------------------------

local _M = {}


local function get_sni(host)
    if not host or host == "" then
        return nil
    end
    local sni = string.lower(host)
    -- TODO: strip possible port, strip ipv6 bracket
    return sni
end


-- Perform the request specified by <req>, while the <options> control
-- the behaviors.
--
-- Request object <req>:
-- + scheme: "http" or "https"
-- + server: { host, port } (host may be ip or domain name)
-- + method: "GET", "POST", etc.
-- + path: URL path
-- + query: query parameters
-- + headers: (table) list of headers
-- + body: (string/array) request body
--
-- Object <options>:
-- + ssl_verify: (bool) whether to perform SSL verification? (default: false)
-- + proxy: (TODO)
--
-- Return: <res>, <err>
-- Result object <res>:
-- + status: (number) status code
-- + headers: (table) response headers
-- + body: (string) response body
--
function _M.request(req, options)
    options = options or {}

    local client, err = Client.new()
    if not client then
        return nil, err
    end

    local host, port = req.server[1], req.server[2]
    local is_https = (req.scheme == "https")
    local headers = Headers.new(req.headers)
    local sni = get_sni(headers["Host"])

    -- Construct a pool name unique with server and SSL info.
    local pool_name = req.scheme .. ":" .. host .. ":" .. tostring(port) ..
                      tostring(is_https) .. ":" .. (sni or "")
    local opts = {
        pool_name = pool_name,
        ssl = is_https,
        ssl_verify = options.ssl_verify,
        ssl_server_name = sni,
    }
    local ok, err = client:connect(host, port, opts)
    if not ok then
        client:close()
        return nil, err
    end

    -- Prepare the Host header if missing.
    if not headers["Host"] then
        if (not is_https and port == 80) or
           (is_https and port == 443)
        then
            headers["Host"] = host
        else
            headers["Host"] = host .. ":" .. tostring(port)
        end
    end
    local req2 = {
        method = req.method or "GET",
        path = req.path or "/",
        query = req.query,
        headers = headers,
        body = req.body,
    }
    ok, err = client:send_request(req2)
    if not ok then
        client:close()
        return nil, err
    end

    local res, err = client:read_response(req2)
    if not res then
        client:close()
        return nil, err
    end

    client:set_keepalive()
    return res, err
end


setmetatable(_M, {
    __newindex = function ()
        error("modification forbidden", 2)
    end,
})

return _M
