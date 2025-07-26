// SPDX-License-Identifier: MIT
//
// Copyright (c) 2025 Aaron LI
//
// Implement the proxy function.
//

package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type WikiProxy struct {
	// The proxy<->target domains.
	// The proxy domain may be wildcard that begins with a '*'.
	domains map[string]string
	// The proxy handler to visit Wikipedia.
	handler *httputil.ReverseProxy
	// path prefix used in translating domains
	transPrefix string
}

// If the <proxy> is specified, it will be used to proxy the Wikipedia
// requests.
func NewWikiProxy(proxy string) (*WikiProxy, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		pURL, err := url.Parse(proxy)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(pURL)
		slog.Info("use proxy", "url", pURL)
	}

	wp := WikiProxy{
		domains:     make(map[string]string),
		transPrefix: "/_wp_/",
	}
	wp.handler = &httputil.ReverseProxy{
		Rewrite:        wp.rewrite,
		ModifyResponse: wp.modifyResponse,
		Transport:      transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Error("proxy error", "host", r.Host,
				"method", r.Method, "url", r.URL,
				"header", r.Header, "error", err)
			if errors.Is(err, context.Canceled) {
				http.Error(w, "gateway timeout",
					http.StatusGatewayTimeout)
			} else {
				http.Error(w, "bad gateway timeout",
					http.StatusBadGateway)
			}
		},
	}

	return &wp, nil
}

// Add <domain> for proxying to the Wikipedia site.
func (wp *WikiProxy) AddDomain(language string, domain string) {
	var site string
	switch language {
	case "en", "english", "English":
		site = "en.wikipedia.org"
	case "zh", "chinese", "Chinese":
		site = "zh.wikipedia.org"
	default:
		panic(fmt.Sprintf("unsupported language %s", language))
	}

	wp.domains[domain] = site
	slog.Info("added proxying domain", "domain", domain, "language", language,
		"site", site)
}

// Information about the original request that needed in ModifyResponse().
// Passed through the request as a context value.
type wpRequestInfo struct {
	// The main domain of the Wikipedia site.
	// e.g., en.wikipedia.org
	site string
	// The target URL to visit.
	target *url.URL
	// The request host; prefer "X-Forwarded-Host" if present.
	// e.g., en.wikiproxy.org:2012
	host string
	// The request scheme; prefer "X-Forwarded-Proto" if present.
	// e.g., https
	scheme string
}

func (wp *WikiProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slog.Debug("request", "host", r.Host, "url", r.URL, "header", r.Header,
		"tls", r.TLS != nil)
	site := wp.getSite(r.Host)
	if site == "" {
		slog.Info("service not found", "host", r.Host)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	target := wp.getTarget(r.URL, site)
	if target == nil {
		slog.Info("invalid request", "url", r.URL)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	slog.Debug("request", "site", site, "target_url", target)

	reqInfo := &wpRequestInfo{
		site:   site,
		target: target,
		host:   r.Header.Get("X-Forwarded-Host"),
		scheme: r.Header.Get("X-Forwarded-Proto"),
	}
	if reqInfo.host == "" {
		reqInfo.host = r.Host
	}
	if reqInfo.scheme == "" {
		if r.TLS != nil {
			reqInfo.scheme = "https"
		} else {
			reqInfo.scheme = "http"
		}
	}
	slog.Debug("extra request info", "value", reqInfo)

	ctx := context.WithValue(r.Context(), "wpRequestInfo", reqInfo)
	r = r.WithContext(ctx)

	wp.handler.ServeHTTP(w, r)
}

// Get the main domain of the target Wikipedia site.
// e.g., en.wikipedia.org
func (wp *WikiProxy) getSite(host string) string {
	// The <host> directly comes from the 'Host' request header, so need
	// to normalize it.
	domain := strings.ToLower(host)
	if strings.Contains(domain, ":") {
		domain, _, _ = net.SplitHostPort(domain)
	}
	slog.Debug("incoming request", "host", host, "domain", domain)

	if site, ok := wp.domains[domain]; ok {
		return site
	}

	// Fallback to check wildcard domains.
	for pattern, site := range wp.domains {
		if strings.HasPrefix(pattern, "*.") &&
			strings.HasSuffix(domain, pattern[1:]) {
			return site
		}
	}

	return ""
}

func (wp *WikiProxy) getTarget(reqURL *url.URL, site string) *url.URL {
	host := site
	path := reqURL.Path
	if strings.HasPrefix(path, wp.transPrefix) {
		// Extract the target domain from the translated path:
		// "/<transPrefix>/<target>/<origPath>"
		path = strings.TrimPrefix(path, wp.transPrefix)
		if i := strings.Index(path, "/"); i > 0 {
			host = path[:i]
			path = path[i:]
			slog.Debug("extracted target", "host", host, "path", path)
		} else {
			slog.Debug("invalid translated path", "path", path)
			return nil
		}
	}

	domains := []string{
		"wikipedia.org",
		"*.wikipedia.org",
		"wikimedia.org",
		"*.wikimedia.org",
	}
	valid := false
	for _, d := range domains {
		if strings.HasPrefix(d, "*.") {
			if strings.HasSuffix(host, d[1:]) {
				valid = true
				break
			}
		} else {
			if d == host {
				valid = true
				break
			}
		}
	}
	if !valid {
		slog.Debug("invalid host", "host", host)
		return nil
	}

	return &url.URL{
		Scheme:   "https",
		Host:     host,
		Path:     path,
		RawQuery: reqURL.RawQuery,
	}
}

// Callback of ReverseProxy to rewrite the request.
func (wp *WikiProxy) rewrite(r *httputil.ProxyRequest) {
	ctx := r.In.Context()
	reqInfo := ctx.Value("wpRequestInfo").(*wpRequestInfo)

	r.Out.URL = reqInfo.target
	r.Out.Host = "" // so will use r.Out.URL.Host
	slog.Debug("set target", "url", r.Out.URL)

	if strings.Contains(r.In.Header.Get("Accept-Encoding"), "gzip") {
		// modifyResponse() only supports gzip.
		r.Out.Header.Set("Accept-Encoding", "gzip")
	}

	// Simply remove the 'Referer' header: (1) avoid blocking based on
	// referrer sites; (2) improve privacy.
	r.Out.Header.Del("Referer")
}

// Callback of ReverseProxy to modify the response.
func (wp *WikiProxy) modifyResponse(resp *http.Response) error {
	ctx := resp.Request.Context()
	reqInfo := ctx.Value("wpRequestInfo").(*wpRequestInfo)

	if err := wp.modifyBody(resp, reqInfo); err != nil {
		return err
	}

	if location := resp.Header.Get("Location"); location != "" {
		newLoc := wp.translateURLs([]byte(location), reqInfo)
		resp.Header.Set("Location", string(newLoc))
	}
	if refresh := resp.Header.Get("Refresh"); refresh != "" {
		newRefresh := wp.translateURLs([]byte(refresh), reqInfo)
		resp.Header.Set("Refresh", string(newRefresh))
	}

	// Fix Set-Cookie headers.
	cookies := resp.Cookies()
	resp.Header.Del("Set-Cookie")
	for _, c := range cookies {
		c.Domain = ""
		c.Secure = (reqInfo.scheme == "https")
		resp.Header.Add("Set-Cookie", c.String())
	}

	// Delete some unwanted headers.
	resp.Header.Del("Strict-Transport-Security")
	resp.Header.Del("Report-To")
	resp.Header.Del("Reporting-Endpoints")
	resp.Header.Del("Nel") // Network Error Logging
	resp.Header.Del("X-Client-Ip")
	resp.Header.Del("Transfer-Encoding") // prevent from chunking

	return nil
}

func (wp *WikiProxy) modifyBody(resp *http.Response, reqInfo *wpRequestInfo) error {
	// Wikipedia may return a 304 (Not Modified) with an empty body but
	// setting 'Content-Encoding: gzip', which would cause gzip.NewReader()
	// to fail with error=EOF.  So check the content length first.
	if resp.ContentLength == 0 {
		return nil
	}

	// Content-Type may be missing, especially in a redirection response
	// without body.
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil {
			slog.Warn("invalid Content-Type", "value", contentType,
				"error", err)
		} else {
			contentType = mediaType
		}
	}

	modifyMIMEs := []string{
		"text/html",
		"text/javascript", // dynamically loaded contents
		"text/css",        // url() in background attribute, etc.
	}
	doModify := false
	for _, ct := range modifyMIMEs {
		if contentType == ct {
			doModify = true
			break
		}
	}
	if !doModify {
		return nil
	}

	// slog.Debug("origin response", "response", resp, "request", resp.Request)

	var body []byte
	var err error
	defer resp.Body.Close()

	switch encoding := resp.Header.Get("Content-Encoding"); encoding {
	case "", "identity":
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("failed to read body", "error", err,
				"response", resp, "request", resp.Request)
			return errors.New("body read failure")
		}
	case "gzip":
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			slog.Error("failed to create gzip reader", "error", err,
				"response", resp, "request", resp.Request)
			return errors.New("gzip body read failure")
		}
		body, err = io.ReadAll(gr)
		gr.Close()
		if err != nil {
			slog.Error("failed to read gzip body", "error", err,
				"response", resp, "request", resp.Request)
			return errors.New("gzip body read failure")
		}
	default:
		// Can only be plain text or gzip, as we enforced in Rewrite().
		panic("impossible Content-Encoding: " + encoding)
	}

	newBody := wp.translateURLs(body, reqInfo)
	slog.Debug("translated body", "content_type", contentType,
		"old_length", len(body), "new_length", len(newBody))

	resp.Body = io.NopCloser(bytes.NewBuffer(newBody))
	resp.Header.Set("Content-Length", strconv.Itoa(len(newBody)))
	resp.Header.Del("Content-Encoding")

	return nil
}

// Regex to match Wikipedia URLs in order to perform translation.
//
// Domains:
// - wikipedia.org
// - *.wikipedia.org
// - wikimedia.org
// - *.wikimedia.org
//
// NOTE: Go's regex doesn't support lookahead and lookbehind matches, so use
// the <prefix> group to workaround it.
// NOTE: JSON script may escape '/' as '\/';
// e.g., {"url": "https:\/\/www.wikimedia.org\/static\/...",...}
var reWikiURL = regexp.MustCompile(`(?P<prefix>^|[^\w:/])` +
	`(?P<scheme>\bhttps?:)?` +
	`(?P<slashes>\\?/\\?/)` +
	`(?P<domain>(?:\w+\.)*(?:wikipedia\.org|wikimedia\.org))\b`)

func (wp *WikiProxy) translateURLs(input []byte, reqInfo *wpRequestInfo) []byte {
	if len(input) == 0 {
		return input
	}

	re := reWikiURL
	giPrefix := re.SubexpIndex("prefix")
	giScheme := re.SubexpIndex("scheme")
	giSlashes := re.SubexpIndex("slashes")
	giDomain := re.SubexpIndex("domain")

	output := bytes.Buffer{}
	last := 0

	inputLC := bytes.ToLower(input) // ignore case in regex matching
	for _, match := range re.FindAllSubmatchIndex(inputLC, -1) {
		output.Write(input[last:match[0]])
		last = match[1]

		slashes := string(inputLC[match[giSlashes*2]:match[giSlashes*2+1]])
		domain := string(inputLC[match[giDomain*2]:match[giDomain*2+1]])
		scheme := ""
		if match[giScheme*2] >= 0 {
			// The <scheme> group may not match ('?' repetition).
			scheme = string(inputLC[match[giScheme*2]:match[giScheme*2+1]])
		}

		repl := bytes.Buffer{}
		repl.Write(input[match[giPrefix*2]:match[giPrefix*2+1]])
		if scheme != "" {
			repl.WriteString(reqInfo.scheme + ":")
		}
		repl.WriteString(slashes)
		repl.WriteString(reqInfo.host)
		if domain != reqInfo.site {
			tPrefix := wp.transPrefix
			if slashes != "//" {
				tPrefix = strings.ReplaceAll(tPrefix, "/", `\/`)
			}
			repl.WriteString(tPrefix)
			repl.WriteString(domain)
		}
		output.Write(repl.Bytes())

		// slog.Debug("translated", "from", string(input[match[0]:match[1]]), "to", repl.String())
	}

	output.Write(input[last:])
	return output.Bytes()
}
