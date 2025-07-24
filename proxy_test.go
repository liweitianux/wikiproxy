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

func TestTranslateURLs(t *testing.T) {
	wp := &WikiProxy{transPrefix: "/_wp_/"}
	info := &wpRequestInfo{
		site:   "en.wikipedia.org",
		host:   "en.wikiproxy.org:2012",
		scheme: "http",
	}

	testcases := []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "empty",
			input:  "",
			output: "",
		},
		{
			name:   "nomatch",
			input:  "A simple Wikipedia proxy in Go.",
			output: "A simple Wikipedia proxy in Go.",
		},
		{
			name:   "site1",
			input:  "https://en.wikipedia.org",
			output: "http://en.wikiproxy.org:2012",
		},
		{
			name:   "location1",
			input:  "Location: https://en.wikipedia.org/wiki/Main_Page",
			output: "Location: http://en.wikiproxy.org:2012/wiki/Main_Page",
		},
		{
			name:   "header1",
			input:  `<link rel="alternate" media="only screen and (max-width: 640px)" href="//en.m.wikipedia.org/wiki/Main_Page">`,
			output: `<link rel="alternate" media="only screen and (max-width: 640px)" href="//en.wikiproxy.org:2012/_wp_/en.m.wikipedia.org/wiki/Main_Page">`,
		},
		{
			name:   "header2",
			input:  `<link rel="dns-prefetch" href="//meta.wikimedia.org" />`,
			output: `<link rel="dns-prefetch" href="//en.wikiproxy.org:2012/_wp_/meta.wikimedia.org" />`,
		},
		{
			name:   "header3",
			input:  `<link rel="preconnect" href="//upload.wikimedia.org">`,
			output: `<link rel="preconnect" href="//en.wikiproxy.org:2012/_wp_/upload.wikimedia.org">`,
		},
		{
			name:   "content1",
			input:  `<li id="pt-sitesupport-2" class="user-links-collapsible-item mw-list-item user-links-collapsible-item"><a data-mw="interface" href="https://donate.wikimedia.org/?wmf_source=donate&amp;wmf_medium=sidebar&amp;wmf_campaign=en.wikipedia.org&amp;uselang=en" class=""><span>Donate</span></a>`,
			output: `<li id="pt-sitesupport-2" class="user-links-collapsible-item mw-list-item user-links-collapsible-item"><a data-mw="interface" href="http://en.wikiproxy.org:2012/_wp_/donate.wikimedia.org/?wmf_source=donate&amp;wmf_medium=sidebar&amp;wmf_campaign=en.wikipedia.org&amp;uselang=en" class=""><span>Donate</span></a>`,
		},
		{
			name:   "multi1",
			input:  `<div><span typeof="mw:File"><a href="https://commons.wikimedia.org/wiki/" title="Commons"><img alt="Commons logo" src="//upload.wikimedia.org/wikipedia/en/thumb/4/4a/Commons-logo.svg/40px-Commons-logo.svg.png" decoding="async" width="31" height="42" class="mw-file-element" srcset="//upload.wikimedia.org/wikipedia/en/thumb/4/4a/Commons-logo.svg/60px-Commons-logo.svg.png 1.5x, //upload.wikimedia.org/wikipedia/en/thumb/4/4a/Commons-logo.svg/120px-Commons-logo.svg.png 2x" data-file-width="1024" data-file-height="1376"></a></span></div>`,
			output: `<div><span typeof="mw:File"><a href="http://en.wikiproxy.org:2012/_wp_/commons.wikimedia.org/wiki/" title="Commons"><img alt="Commons logo" src="//en.wikiproxy.org:2012/_wp_/upload.wikimedia.org/wikipedia/en/thumb/4/4a/Commons-logo.svg/40px-Commons-logo.svg.png" decoding="async" width="31" height="42" class="mw-file-element" srcset="//en.wikiproxy.org:2012/_wp_/upload.wikimedia.org/wikipedia/en/thumb/4/4a/Commons-logo.svg/60px-Commons-logo.svg.png 1.5x, //en.wikiproxy.org:2012/_wp_/upload.wikimedia.org/wikipedia/en/thumb/4/4a/Commons-logo.svg/120px-Commons-logo.svg.png 2x" data-file-width="1024" data-file-height="1376"></a></span></div>`,
		},
		{
			name:   "json1",
			input:  `<script type="application/ld+json">{"@context":"https:\/\/schema.org","@type":"Article","name":"Main Page","url":"https:\/\/en.wikipedia.org\/wiki\/Main_Page","sameAs":"http:\/\/www.wikidata.org\/entity\/Q5296","mainEntity":"http:\/\/www.wikidata.org\/entity\/Q5296","author":{"@type":"Organization","name":"Contributors to Wikimedia projects"},"publisher":{"@type":"Organization","name":"Wikimedia Foundation, Inc.","logo":{"@type":"ImageObject","url":"https:\/\/www.wikimedia.org\/static\/images\/wmf-hor-googpub.png"}},"datePublished":"2002-01-26T15:28:12Z","dateModified":"2025-07-05T04:58:10Z","image":"https:\/\/upload.wikimedia.org\/wikipedia\/commons\/1\/16\/Great_Wilbraham_site_map.png","headline":"Wikimedia project page"}</script>`,
			output: `<script type="application/ld+json">{"@context":"https:\/\/schema.org","@type":"Article","name":"Main Page","url":"http:\/\/en.wikiproxy.org:2012\/wiki\/Main_Page","sameAs":"http:\/\/www.wikidata.org\/entity\/Q5296","mainEntity":"http:\/\/www.wikidata.org\/entity\/Q5296","author":{"@type":"Organization","name":"Contributors to Wikimedia projects"},"publisher":{"@type":"Organization","name":"Wikimedia Foundation, Inc.","logo":{"@type":"ImageObject","url":"http:\/\/en.wikiproxy.org:2012\/_wp_\/www.wikimedia.org\/static\/images\/wmf-hor-googpub.png"}},"datePublished":"2002-01-26T15:28:12Z","dateModified":"2025-07-05T04:58:10Z","image":"http:\/\/en.wikiproxy.org:2012\/_wp_\/upload.wikimedia.org\/wikipedia\/commons\/1\/16\/Great_Wilbraham_site_map.png","headline":"Wikimedia project page"}</script>`,
		},
	}

	for _, tc := range testcases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			output := wp.translateURLs([]byte(tc.input), info)
			if string(output) != tc.output {
				t.Errorf("wrong output: %q\nexpected: %q",
					string(output), tc.output)
			}
		})
	}
}
