// SPDX-License-Identifier: MIT
//
// Copyright (c) 2025 Aaron LI

package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time {
	return c.now
}

func TestAuthenticator(t *testing.T) {
	clock := fakeClock{now: time.Now()}
	auth := Authenticator{
		Retries:  3,
		WaitTime: 10,
		TTL:      3600,
		clock:    &clock,
	}
	handler := auth.Middleware(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("authenticated"))
		}))

	ts := httptest.NewServer(handler)
	defer ts.Close()

	t.Run("unique_sid", func(t *testing.T) {
		getSID := func(url string) (string, error) {
			resp, err := http.Get(url)
			if err != nil {
				return "", fmt.Errorf("request failed: %v", err)
			}

			resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				return "", fmt.Errorf("expected status 404, got %d",
					resp.StatusCode)
			}

			var cookie *http.Cookie
			for _, c := range resp.Cookies() {
				if c.Name == "wikiproxy" {
					cookie = c
					break
				}
			}
			if cookie == nil {
				return "", errors.New("cookie not found")
			}

			info, err := auth.parseCookie(cookie.Value)
			if err != nil {
				return "", fmt.Errorf("invalid cookie: %v", err)
			}

			return info.SID, nil
		}

		sid1, err := getSID(ts.URL)
		if err != nil {
			t.Fatalf("first request failed: %v", err)
		}
		sid2, err := getSID(ts.URL)
		if err != nil {
			t.Fatalf("second request failed: %v", err)
		}
		if sid1 == sid2 {
			t.Fatalf("two requests have the same sid")
		}
	})

	t.Run("invalid_cookie", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
		req.Header.Set("Cookie", "wikiproxy=invalid")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}

		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d", resp.StatusCode)
		}
	})

	t.Run("authflow", func(t *testing.T) {
		jar, _ := cookiejar.New(nil)
		client := &http.Client{Jar: jar}

		requests := []struct {
			code int
			body string
			// seconds to add to clock to simulate time passing
			addtime int
		}{
			{code: 404},
			{code: 404},
			{
				code: 404,
				// expire the cookie to restart auth
				addtime: auth.WaitTime + 1,
			},
			{code: 404},
			{
				code: 404,
				// cookie still valid
				addtime: auth.WaitTime - 1,
			},
			{code: 200, body: "authenticated"},
			{code: 200, body: "authenticated"},
			{
				code: 200,
				// cookie still valid
				addtime: auth.TTL - 1,
			},
			{
				code: 404,
				// cookie expired
				addtime: 1,
			},
			{code: 404},
		}
		for i, req := range requests {
			if req.addtime > 0 {
				clock.now = clock.now.Add(
					time.Duration(req.addtime) * time.Second)
				t.Logf("time passed %d seconds", req.addtime)
			}

			resp, err := client.Get(ts.URL)
			if err != nil {
				t.Fatalf("#%d request failed: %v", i+1, err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != req.code {
				t.Fatalf("#%d request expected code %d, got %d",
					i+1, req.code, resp.StatusCode)
			}
			if req.body != "" && string(body) != req.body {
				t.Fatalf("#%d request expected body %q, got %q",
					i+1, req.body, string(body))
			}
		}
	})
}
