// SPDX-License-Identifier: MIT
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
	"slices"
	"regexp"
	"strconv"
	"strings"
)

// List of domains used by Wikipedia.
var wikiDomains = []string{
	/* main domains */
	"en.wikipedia.org",
	"zh.wikipedia.org",
	/* mobile domains */
	"en.m.wikipedia.org",
	"zh.m.wikipedia.org",
	/* assets domains */
	"wikimedia.org",
	"commons.wikimedia.org",
	"login.wikimedia.org",
	"meta.wikimedia.org",
	"upload.wikimedia.org",
	"www.wikimedia.org",
}

type WikiProxy struct {
	// The proxy<->target domains.
	// The proxy domain may be wildcard that begins with a '*'.
	domains map[string]string
	// Per-domain proxy handlers.
	handlers map[string]*httputil.ReverseProxy
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
		handlers:    make(map[string]*httputil.ReverseProxy),
		transPrefix: "/_wp_/",
	}
	for _, domain := range wikiDomains {
		tURL := &url.URL{
			Scheme: "https",
			Host:   domain,
		}
		wp.handlers[domain] = &httputil.ReverseProxy{
			Rewrite: func(r *httputil.ProxyRequest) {
				r.SetURL(tURL)
				slog.Debug("set target", "url", r.Out.URL)
				// modifyResponse() always supports gzip.
				r.Out.Header.Set("Accept-Encoding", "gzip")
			},
			ModifyResponse: wp.modifyResponse,
			Transport:      transport,
		}
	}

	return &wp, nil
}

// Add <domain> for proxying to the Wikipedia site.
func (wp *WikiProxy) AddDomain(language string, domain string) {
	var target string
	switch language {
	case "en", "english", "English":
		target = "en.wikipedia.org"
	case "zh", "chinese", "Chinese":
		target = "zh.wikipedia.org"
	default:
		panic(fmt.Sprintf("unsupported language %s", language))
	}
	if slices.Index(wikiDomains, target) < 0 {
		panic(fmt.Sprintf("wikiDomains missing target: %s", target))
	}

	wp.domains[domain] = target
	slog.Info("added proxying domain", "domain", domain, "language", language,
		"target", target)
}

// Information about the original request that needed in ModifyResponse().
// Passed through the request as a context value.
type wpRequestInfo struct {
	// The main domain of the Wikipedia site.
	// e.g., en.wikipedia.org
	target string
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
	target := wp.getTarget(r.Host)
	if target == "" {
		slog.Info("service not found", "host", r.Host)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	path := r.URL.Path
	if strings.HasPrefix(path, wp.transPrefix) {
		// Extract the target domain from the translated path:
		// "/<transPrefix>/<target>/<origPath>"
		path = strings.TrimPrefix(path, wp.transPrefix)
		t, p, found := strings.Cut(path, "/")
		if !found {
			slog.Info("invalid translated path", "path", path)
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		target = t
		path = p
	}
	slog.Debug("incoming request", "req_path", r.URL.Path,
		"target", target, "target_path", path)
	r.URL.Path = path

	handler := wp.handlers[target]
	if handler == nil {
		slog.Info("proxy handler not found", "target", target)
		http.Error(w, "not implemented", http.StatusNotImplemented)
		return
	}

	reqInfo := &wpRequestInfo{
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

	handler.ServeHTTP(w, r)
}

func (wp *WikiProxy) getTarget(host string) string {
	// The <host> directly comes from the 'Host' request header, so need
	// to normalize it.
	domain := strings.ToLower(host)
	if strings.Contains(domain, ":") {
		domain, _, _ = net.SplitHostPort(domain)
	}
	slog.Debug("incoming request", "host", host, "domain", domain)

	if target, ok := wp.domains[domain]; ok {
		return target
	}

	// Fallback to check wildcard domains.
	for pattern, target := range wp.domains {
		if strings.HasPrefix(pattern, "*.") &&
			strings.HasSuffix(domain, pattern[1:]) {
			return target
		}
	}

	return ""
}

func (wp *WikiProxy) modifyResponse(resp *http.Response) error {
	ctx := resp.Request.Context()
	reqInfo := ctx.Value("wpRequestInfo").(*wpRequestInfo)

	contentType := resp.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		slog.Error("invalid Content-Type", "value", contentType, "error", err)
		return errors.New("invalid Content-Type")
	}

	// slog.Debug("origin response", "response", resp, "request", resp.Request)

	// text/css: deal with url() in background attribute, etc.
	if mediaType == "text/html" ||
		mediaType == "text/javascript" ||
		mediaType == "text/css" {

		encoding := resp.Header.Get("Content-Encoding")
		defer resp.Body.Close()

		var body []byte
		if encoding == "gzip" {
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
		} else {
			// Assume plain text, as we enforced in Rewrite().
			body, err = io.ReadAll(resp.Body)
			if err != nil {
				slog.Error("failed to read body", "error", err,
					"response", resp, "request", resp.Request)
				return errors.New("body read failure")
			}
		}

		newBody := wp.translateURLs(body, reqInfo)
		slog.Debug("translated body", "media_type", mediaType,
			"old_length", len(body), "new_length", len(newBody))

		resp.Body = io.NopCloser(bytes.NewBuffer(newBody))
		resp.Header.Set("Content-Length", strconv.Itoa(len(newBody)))
		resp.Header.Del("Content-Encoding")

	}

	// TODO: Deal with headers: Set-Cookie, Referer, Origin, Refresh, ...
	if location := resp.Header.Get("Location"); location != "" {
		newLoc := wp.translateURLs([]byte(location), reqInfo)
		resp.Header.Set("Location", string(newLoc))
	}
	if refresh := resp.Header.Get("Refresh"); refresh != "" {
		newRefresh := wp.translateURLs([]byte(refresh), reqInfo)
		resp.Header.Set("Refresh", string(newRefresh))
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
		if domain != reqInfo.target {
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
