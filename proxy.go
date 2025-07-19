// SPDX-License-Identifier: MIT
//
// Implement the proxy function.
//

package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
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
	// TODO
	return nil
}
