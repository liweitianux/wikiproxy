// SPDX-License-Identifier: MIT
//
// Copyright (c) 2025 Aaron LI

package main

import (
	"testing"
)

func TestReWikiURL(t *testing.T) {
	type testcase struct {
		name        string
		content     string
		shouldMatch bool
		prefix      string
		scheme      string
		slashes     string
		domain      string
	}
	testcases := []*testcase{
		// match cases
		{
			name:        "simple1",
			content:     "https://www.wikipedia.org/",
			shouldMatch: true,
			scheme:      "https:",
			slashes:     "//",
			domain:      "www.wikipedia.org",
		},
		{
			name:        "simple2",
			content:     "https://www.wikimedia.org/",
			shouldMatch: true,
			scheme:      "https:",
			slashes:     "//",
			domain:      "www.wikimedia.org",
		},
		{
			name:        "nopath",
			content:     "https://www.wikipedia.org",
			shouldMatch: true,
			scheme:      "https:",
			slashes:     "//",
			domain:      "www.wikipedia.org",
		},
		{
			name:        "prefix1",
			content:     " https://www.wikimedia.org/",
			shouldMatch: true,
			prefix:      " ",
			scheme:      "https:",
			slashes:     "//",
			domain:      "www.wikimedia.org",
		},
		{
			name:        "prefix2",
			content:     "=https://www.wikimedia.org/",
			shouldMatch: true,
			prefix:      "=",
			scheme:      "https:",
			slashes:     "//",
			domain:      "www.wikimedia.org",
		},
		{
			name:        "path",
			content:     "https://www.wikipedia.org/a/b/c",
			shouldMatch: true,
			scheme:      "https:",
			slashes:     "//",
			domain:      "www.wikipedia.org",
		},
		{
			name:        "query1",
			content:     "https://www.wikipedia.org?",
			shouldMatch: true,
			scheme:      "https:",
			slashes:     "//",
			domain:      "www.wikipedia.org",
		},
		{
			name:        "query2",
			content:     "https://www.wikipedia.org?a=b",
			shouldMatch: true,
			scheme:      "https:",
			slashes:     "//",
			domain:      "www.wikipedia.org",
		},
		{
			name:        "query3",
			content:     "https://www.wikipedia.org/?a=b&c",
			shouldMatch: true,
			scheme:      "https:",
			slashes:     "//",
			domain:      "www.wikipedia.org",
		},
		{
			name:        "path+query",
			content:     "https://www.wikipedia.org/x/y?a=b&c",
			shouldMatch: true,
			scheme:      "https:",
			slashes:     "//",
			domain:      "www.wikipedia.org",
		},
		{
			name:        "fragment1",
			content:     "https://www.wikipedia.org#",
			shouldMatch: true,
			scheme:      "https:",
			slashes:     "//",
			domain:      "www.wikipedia.org",
		},
		{
			name:        "fragment2",
			content:     "https://www.wikipedia.org#abc",
			shouldMatch: true,
			scheme:      "https:",
			slashes:     "//",
			domain:      "www.wikipedia.org",
		},
		{
			name:        "path+fragment2",
			content:     "https://www.wikipedia.org/x/yz#abc",
			shouldMatch: true,
			scheme:      "https:",
			slashes:     "//",
			domain:      "www.wikipedia.org",
		},
		{
			name:        "http",
			content:     "http://www.wikipedia.org",
			shouldMatch: true,
			scheme:      "http:",
			slashes:     "//",
			domain:      "www.wikipedia.org",
		},
		{
			name:        "noscheme",
			content:     "//www.wikipedia.org",
			shouldMatch: true,
			scheme:      "",
			slashes:     "//",
			domain:      "www.wikipedia.org",
		},
		{
			name:        "apex1",
			content:     "https://wikipedia.org/",
			shouldMatch: true,
			scheme:      "https:",
			slashes:     "//",
			domain:      "wikipedia.org",
		},
		{
			name:        "apex2",
			content:     "https://wikimedia.org/",
			shouldMatch: true,
			scheme:      "https:",
			slashes:     "//",
			domain:      "wikimedia.org",
		},
		{
			name:        "subdomain1",
			content:     "https://en.m.wikipedia.org/",
			shouldMatch: true,
			scheme:      "https:",
			slashes:     "//",
			domain:      "en.m.wikipedia.org",
		},
		{
			name:        "href1",
			content:     `<link rel="EditURI" type="application/rsd+xml" href="//en.wikipedia.org/w/api.php?action=rsd">`,
			shouldMatch: true,
			prefix:      `"`,
			scheme:      "",
			slashes:     "//",
			domain:      "en.wikipedia.org",
		},
		{
			name:        "href2",
			content:     `<link rel="canonical" href="https://en.wikipedia.org/wiki/Main_Page">`,
			shouldMatch: true,
			prefix:      `"`,
			scheme:      "https:",
			slashes:     "//",
			domain:      "en.wikipedia.org",
		},
		{
			name:        "href3",
			content:     `<li><a class="external text" href="https://de.wikipedia.org/wiki/"><span class="autonym" title="German (de:)" lang="de">Deutsch</span></a></li>`,
			shouldMatch: true,
			prefix:      `"`,
			scheme:      "https:",
			slashes:     "//",
			domain:      "de.wikipedia.org",
		},
		{
			name:        "div1",
			content:     `...>https://en.wikipedia.org/w/index.php?title=Main_Page&amp;oldid=1298856702</a>"</div></div>`,
			shouldMatch: true,
			prefix:      `>`,
			scheme:      "https:",
			slashes:     "//",
			domain:      "en.wikipedia.org",
		},
		{
			name:        "json1",
			content:     `https:\/\/en.wikipedia.org/`,
			shouldMatch: true,
			scheme:      "https:",
			slashes:     `\/\/`,
			domain:      "en.wikipedia.org",
		},
		{
			name:        "json2",
			content:     `<script type="application/ld+json">{"@context":"https:\/\/schema.org","@type":"Article","name":"Main Page","url":"https:\/\/en.wikipedia.org\/wiki\/Main_Page",...`,
			shouldMatch: true,
			prefix:      `"`,
			scheme:      "https:",
			slashes:     `\/\/`,
			domain:      "en.wikipedia.org",
		},
		// no match cases
		{
			name:        "wrongdomain1",
			content:     "https://www.example.org/",
			shouldMatch: false,
		},
		{
			name:        "wrongdomain2",
			content:     "https://www.wikipedia.com/",
			shouldMatch: false,
		},
		{
			name:        "wildcard",
			content:     "https://*.wikipedia.org/",
			shouldMatch: false,
		},
		{
			name:        "dot1",
			content:     "https://.wikipedia.org/",
			shouldMatch: false,
		},
		{
			name:        "dotdot1",
			content:     "https://..wikipedia.org/",
			shouldMatch: false,
		},
		{
			name:        "dotdot2",
			content:     "https://www..wikipedia.org/",
			shouldMatch: false,
		},
		{
			name:        "wrongscheme1",
			content:     "xhttps://www.wikipedia.org/",
			shouldMatch: false,
		},
		{
			name:        "wrongscheme2",
			content:     "xxx://www.wikipedia.org/",
			shouldMatch: false,
		},
		{
			name:        "wrongscheme3",
			content:     "://www.wikipedia.org/",
			shouldMatch: false,
		},
		{
			name:        "wrongscheme4",
			content:     "https:://www.wikipedia.org/",
			shouldMatch: false,
		},
		{
			name:        "wrongscheme5",
			content:     "xxx//www.wikipedia.org/",
			shouldMatch: false,
		},
		{
			name:        "wrongslash1",
			content:     "https:///www.wikipedia.org/",
			shouldMatch: false,
		},
		{
			name:        "wrongslash2",
			content:     "https:/www.wikipedia.org/",
			shouldMatch: false,
		},
		{
			name:        "link_noscheme",
			content:     `<link rel="dns-prefetch" href="auth.wikimedia.org">`,
			shouldMatch: false,
		},
	}

	re := reWikiURL
	names := re.SubexpNames()
	t.Logf("regexp names: %q", names)

	for _, tc := range testcases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("matching context: %s", tc.content)
			match := re.FindStringSubmatch(tc.content)
			if match == nil {
				if tc.shouldMatch {
					t.Fatal("should match but found no match")
				}
				return
			}
			if !tc.shouldMatch {
				t.Fatalf("should not match but found match: %q", match)
			}

			// Map names to matched values
			result := make(map[string]string)
			for i, name := range names {
				if i != 0 && name != "" {
					result[name] = match[i]
				}
			}
			t.Logf("match result: %q", result)

			// Check matched values by name
			if prefix := result["prefix"]; prefix != tc.prefix {
				t.Errorf("matched wrong prefix %q, expected %q", prefix, tc.prefix)
			}
			if scheme := result["scheme"]; scheme != tc.scheme {
				t.Errorf("matched wrong scheme %q, expected %q", scheme, tc.scheme)
			}
			if slashes := result["slashes"]; slashes != tc.slashes {
				t.Errorf("matched wrong slashes %q, expected %q", slashes, tc.slashes)
			}
			if domain := result["domain"]; domain != tc.domain {
				t.Errorf("matched wrong domain %q, expected %q", domain, tc.domain)
			}
		})
	}
}
