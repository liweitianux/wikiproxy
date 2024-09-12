-- SPDX-License-Identifier: MIT
--
-- WikiProxy Lua module
-- GZIP compression and decompression routines.
--
-- Credits:
-- * Zlib Usage Example
--   https://zlib.net/zlib_how.html
-- * How can I decompress a gzip stream with zlib?
--   https://stackoverflow.com/a/1838702
-- * lua-ffi-zlib
--   https://github.com/hamishforbes/lua-ffi-zlib
--

local assert = assert
local error = error
local table = table

local ngx = ngx

local default_chunk = 16384 -- 16KB

------------------------------------------------------------------------

local ffi = require("ffi")
local C = ffi.C

-- Derived from zlib.h and zconf.h
-- Version: 1.3.1
ffi.cdef[[
/* Allowed flush values */
enum {
    Z_NO_FLUSH      = 0,
    Z_PARTIAL_FLUSH = 1,
    Z_SYNC_FLUSH    = 2,
    Z_FULL_FLUSH    = 3,
    Z_FINISH        = 4,
    Z_BLOCK         = 5,
    Z_TREES         = 6,
};
/* Return codes for the compression/decompression functions. */
enum {
    Z_OK = 0,
    /* special but normal events */
    Z_STREAM_END = 1,
    Z_NEED_DICT  = 2,
    /* errors */
    Z_ERRNO         = -1,
    Z_STREAM_ERROR  = -2,
    Z_DATA_ERROR    = -3,
    Z_MEM_ERROR     = -4,
    Z_BUF_ERROR     = -5,
    Z_VERSION_ERROR = -6,
};
/* Compression levels */
enum {
    Z_NO_COMPRESSION      = 0,
    Z_BEST_SPEED          = 1,
    Z_BEST_COMPRESSION    = 9,
    Z_DEFAULT_COMPRESSION = -1,
};
/* Compression strategies */
enum {
    Z_FILTERED         = 1,
    Z_HUFFMAN_ONLY     = 2,
    Z_RLE              = 3,
    Z_FIXED            = 4,
    Z_DEFAULT_STRATEGY = 0,
};
/* Compression methods */
enum {
    Z_DEFLATED = 8, /* the only one supported */
};

typedef void *(*alloc_func)(void *opaque, unsigned int items,
                            unsigned int size);
typedef void (*free_func)(void *opaque, void *address);

typedef struct {
    const unsigned char *next_in; /* next input byte */
    unsigned int    avail_in;   /* number of bytes available at next_in */
    unsigned long   total_in;   /* total number of input bytes read so far */

    unsigned char   *next_out;  /* next output byte will go here */
    unsigned int    avail_out;  /* remaining free space at next_out */
    unsigned long   total_out;  /* total number of bytes output so far */

    const char      *msg;       /* last error message, NULL if no error */
    void            *state;     /* not visible by applications */

    alloc_func      zalloc;     /* used to allocate the internal state */
    free_func       zfree;      /* used to free the internal state */
    void            *opaque;    /* private data object passed to zalloc
                                   and zfree */

    int             data_type;  /* best guess about the data type:
                                   binary or text for deflate,
                                   or the decoding state for inflate */
    unsigned long   adler;      /* Adler-32 or CRC-32 value of the
                                   uncompressed data */
    unsigned long   reserved;   /* reserved for future use */
} z_stream;

const char *zError(int);
const char *zlibVersion(void);

/* Add 16 to windowBits for gzip format. */
int deflateInit2_(z_stream *strm, int level, int method, int windowBits,
                  int memLevel, int strategy, const char *version,
                  int stream_size);
int inflateInit2_(z_stream *strm, int windowBits, const char *version,
                  int stream_size);

int deflate(z_stream *strm, int flush);
int deflateEnd(z_stream *strm);

int inflate(z_stream *strm, int flush);
int inflateEnd(z_stream *strm);
]]

local zlib_version = ffi.string(C.zlibVersion())
ngx.log(ngx.DEBUG, "using zlib version: ", zlib_version)

------------------------------------------------------------------------

local _M = {}

function _M.compress(input, level)
    level = level or C.Z_DEFAULT_COMPRESSION

    local stream, rc, err

    stream = ffi.new("z_stream") -- initialized to be zero
    rc = C.deflateInit2_(stream, level,
                         C.Z_DEFLATED, -- method
                         15 + 16, -- windowBits: +16 to use the gzip format
                         8, -- memLevel: default is 8
                         C.Z_DEFAULT_STRATEGY, -- strategy
                         zlib_version,
                         ffi.sizeof(stream))
    if rc ~= C.Z_OK then
        err = ffi.string(C.zError(rc))
        ngx.log(ngx.ERR, "deflateInit2_() failed: ", err)
        return input, err
    end

    local bufsize = default_chunk
    local buf = ffi.new("unsigned char[?]", bufsize)

    stream.next_in = input
    stream.avail_in = #input

    local outputs = {}
    local len
    repeat
        stream.next_out = buf
        stream.avail_out = bufsize

        -- Set flush=C.Z_FINISH since all data are given
        rc = C.deflate(stream, C.Z_FINISH)
        assert(rc ~= C.Z_STREAM_ERROR) -- state not clobbered
        assert(rc ~= C.Z_BUF_ERROR) -- input cannot be empty
        -- no bad return value

        len = bufsize - stream.avail_out
        if len > 0 then
            table.insert(outputs, ffi.string(buf, len))
            --ngx.log(ngx.DEBUG, "deflated chunk of len=", len)
        end
    until rc == C.Z_STREAM_END
    C.deflateEnd(stream)

    local output = table.concat(outputs)
    ngx.log(ngx.DEBUG, "compressed data from ", #input, " to ", #output)
    return output
end

function _M.decompress(input)
    local stream, rc, err

    stream = ffi.new("z_stream") -- initialized to be zero
    rc = C.inflateInit2_(stream,
                         15 + 16, -- windowBits: +16 to decode gzip format
                         zlib_version,
                         ffi.sizeof(stream))
    if rc ~= C.Z_OK then
        err = ffi.string(C.zError(rc))
        ngx.log(ngx.ERR, "inflateInit2_() failed: ", err)
        return input, err
    end

    local bufsize = default_chunk
    local buf = ffi.new("unsigned char[?]", bufsize)

    stream.next_in = input
    stream.avail_in = #input

    local outputs = {}
    local len
    repeat
        stream.next_out = buf
        stream.avail_out = bufsize

        rc = C.inflate(stream, C.Z_NO_FLUSH)
        assert(rc ~= C.Z_STREAM_ERROR) -- state not clobbered
        assert(rc ~= C.Z_BUF_ERROR) -- input cannot be empty
        if rc == C.Z_NEED_DICT or
           rc == C.Z_DATA_ERROR or
           rc == C.Z_MEM_ERROR
        then
            err = ffi.string(C.zError(rc))
            ngx.log(ngx.ERR, "inflate() failed: ", err)
            C.inflateEnd(stream)
            return input, err
        end

        len = bufsize - stream.avail_out
        if len > 0 then
            table.insert(outputs, ffi.string(buf, len))
            --ngx.log(ngx.DEBUG, "inflated chunk of len=", len)
        end
    until rc == C.Z_STREAM_END
    C.inflateEnd(stream)

    local output = table.concat(outputs)
    ngx.log(ngx.DEBUG, "decompressed data from ", #input, " to ", #output)
    return output
end

setmetatable(_M, {
    __newindex = function ()
        error("modification forbidden", 2)
    end,
})

return _M
