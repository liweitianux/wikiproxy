// SPDX-License-Identifier: MIT
//
// Copyright (c) 2025 Aaron LI
//
// Implement a simple authenticator
//

package main

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Authenticator struct {
	// Secret for the HMAC to sign the cookie.
	// If unspecified, then randomly generate one.
	Secret []byte
	// Number of retries to pass the authentication.
	Retries int
	// Time (seconds) to wait for a client to finish authenticating.
	WaitTime int
	// Cache time (seconds) of a successful authentication.
	TTL int

	// HMAC to sign and verify the cookie.
	hmac hash.Hash
	// Mutex to protect hmac from concurrent accesses.
	mutex sync.Mutex
}

type authInfo struct {
	// randomly generated unique session id
	SID string `json:"sid"`
	// remaining tries to go
	Tries int `json:"t"`
	// expire time of the tries (before) or the session (after)
	Expires int64 `json:"exp"`
}

func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	const (
		cookieName   = "wikiproxy"
		cookieMaxAge = 86400 // seconds
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookieSecure := (r.TLS != nil)
		if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
			cookieSecure = (proto == "https")
		}
		setCookie := &http.Cookie{
			Name:     cookieName,
			MaxAge:   cookieMaxAge,
			Secure:   cookieSecure,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Path:     "/",
		}

		cookie, _ := r.Cookie(cookieName)
		if cookie == nil {
			setCookie.Value = a.makeCookie(nil)
			slog.Debug("created cookie", "cookie", setCookie)
			http.SetCookie(w, setCookie)
			http.Error(w, fmt.Sprintf("not found: %d...", a.Retries),
				http.StatusNotFound)
			return
		}
		slog.Debug("got cookie", "cookie", cookie)

		info, err := a.parseCookie(cookie.Value)
		if err != nil {
			slog.Info("invalid cookie", "cookie", cookie, "error", err)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		if info.Expires <= time.Now().Unix() {
			setCookie.Value = a.makeCookie(nil)
			slog.Debug("replaced cookie", "cookie", setCookie)
			http.SetCookie(w, setCookie)
			http.Error(w, fmt.Sprintf("not found: %d...", a.Retries),
				http.StatusNotFound)
			return
		}

		if info.Tries > 0 {
			info.Tries--
			setCookie.Value = a.makeCookie(info)
			slog.Debug("updated cookie", "cookie", setCookie)
			http.SetCookie(w, setCookie)
		}
		if info.Tries > 0 {
			http.Error(w, fmt.Sprintf("not found: %d...", info.Tries),
				http.StatusNotFound)
			return
		}

		// Strip the "wikiproxy" cookie to avoid proxying it.
		cookies := []string{}
		for _, c := range r.Cookies() {
			if c.Name != cookieName {
				cookies = append(cookies, c.String())
			}
		}
		r.Header.Set("Cookie", strings.Join(cookies, "; "))

		next.ServeHTTP(w, r)
	})
}

// Cookie value format: <base64url(json(authInfo))>.<hmac_signature>
func (a *Authenticator) parseCookie(value string) (*authInfo, error) {
	dataStr, sig, found := strings.Cut(value, ".")
	if !found {
		return nil, errors.New("invalid cookie format")
	}
	if a.sign(dataStr) != sig {
		return nil, errors.New("invalid cookie signature")
	}
	data, err := base64.URLEncoding.DecodeString(dataStr)
	if err != nil {
		return nil, errors.New("invalid cookie data")
	}

	info := authInfo{}
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("invalid cookie json: %v", err)
	}

	slog.Debug("parsed authInfo", "value", &info)
	return &info, nil
}

func (a *Authenticator) makeCookie(info *authInfo) string {
	if info == nil {
		info = &authInfo{
			Tries:   a.Retries,
			Expires: time.Now().Unix() + int64(a.WaitTime),
		}
	}
	if info.SID == "" {
		sid := make([]byte, 12)
		_, _ = rand.Read(sid)
		info.SID = base64.URLEncoding.EncodeToString(sid)
	}
	if info.Tries == 0 {
		info.Expires = time.Now().Unix() + int64(a.TTL)
	} else if info.Expires == 0 {
		info.Expires = time.Now().Unix() + int64(a.WaitTime)
	}
	slog.Debug("made authInfo", "value", info)

	data, _ := json.Marshal(info)
	dataStr := base64.URLEncoding.EncodeToString(data)

	return dataStr + "." + a.sign(dataStr)
}

func (a *Authenticator) sign(data string) string {
	if a.hmac == nil {
		secret := a.Secret
		if len(secret) == 0 {
			secret = make([]byte, 33)
			_, err := rand.Read(secret)
			if err != nil {
				// crypto/rand.Read() will always succeed on Go
				// 1.24+, but we're using Go 1.21+ in go.mod.
				panic(err)
			}
			slog.Info("generated a random HMAC secret")
		}
		a.hmac = hmac.New(md5.New, secret)
		slog.Info("created HMAC")
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.hmac.Reset()
	a.hmac.Write([]byte(data))
	result := a.hmac.Sum(nil)

	return hex.EncodeToString(result)
}
