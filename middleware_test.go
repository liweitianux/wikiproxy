// SPDX-License-Identifier: MIT
//
// Copyright (c) 2025 Aaron LI

package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func muteSlog() func() {
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	return func() {
		slog.SetDefault(orig)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	restore := muteSlog()
	defer restore()

	t.Run("no_panic", func(t *testing.T) {
		handler := RecoveryMiddleware(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			}))

		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		resp := rec.Result()
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
		}
		if string(body) != "OK" {
			t.Errorf("Expected body 'OK', got %q", string(body))
		}
	})

	t.Run("panic", func(t *testing.T) {
		handler := RecoveryMiddleware(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				panic("something went wrong")
			}))

		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		resp := rec.Result()
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500 Internal Server Error, got %d", resp.StatusCode)
		}
		if string(body) == "internal server error" {
			t.Errorf("Expected error message in body, got %q", string(body))
		}
	})

	t.Run("ErrAbortHandler", func(t *testing.T) {
		defer func() {
			if r := recover(); r != http.ErrAbortHandler {
				t.Errorf("Expected panic(http.ErrAbortHandler), got: %v", r)
			}
		}()

		handler := RecoveryMiddleware(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				panic(http.ErrAbortHandler)
			}))

		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		t.Fatal("Expected panic, but ServeHTTP returned normally")
	})
}

func TestGzipMiddleware(t *testing.T) {
	tests := []struct {
		name             string
		acceptEncoding   string
		contentType      string
		body             string
		statusCode       int
		expectedEncoding string
		expectCompressed bool
		expectStatus     int
	}{
		{
			name:             "accept_encoding_empty",
			acceptEncoding:   "",
			contentType:      "text/plain",
			body:             "Hello, plain world!",
			expectedEncoding: "",
			expectCompressed: false,
			expectStatus:     200,
		},
		{
			name:             "accept_encoding_br",
			acceptEncoding:   "br",
			contentType:      "text/plain",
			body:             "Hello, plain world!",
			expectedEncoding: "",
			expectCompressed: false,
			expectStatus:     200,
		},
		{
			name:             "gzip_with_compressible",
			acceptEncoding:   "gzip",
			contentType:      "text/plain",
			body:             "Hello, compressed world!",
			expectedEncoding: "gzip",
			expectCompressed: true,
			expectStatus:     200,
		},
		{
			name:             "gzip_uncompressible",
			acceptEncoding:   "gzip",
			contentType:      "image/png",
			body:             "PNGDATA",
			expectedEncoding: "",
			expectCompressed: false,
			expectStatus:     200,
		},
		{
			name:             "301_without_body",
			acceptEncoding:   "gzip",
			contentType:      "",
			body:             "",
			expectedEncoding: "",
			expectCompressed: false,
			expectStatus:     301,
		},
		{
			name:             "301_with_body",
			acceptEncoding:   "gzip",
			contentType:      "text/plain",
			body:             "Redirecting...",
			expectedEncoding: "gzip",
			expectCompressed: true,
			expectStatus:     301,
		},
		{
			name:             "detect_text",
			acceptEncoding:   "gzip",
			contentType:      "",
			body:             "Hello, world!",
			expectedEncoding: "gzip",
			expectCompressed: true,
			expectStatus:     200,
		},
		{
			name:           "detect_binary",
			acceptEncoding: "gzip",
			contentType:    "",
			body: string([]byte{
				0x7f, 0x45, 0x4c, 0x46, 0x02, 0x01, 0x01, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x02, 0x00, 0x3e, 0x00, 0x01, 0x00, 0x00, 0x00,
				0x20, 0x7f, 0x47, 0x00, 0x00, 0x00, 0x00, 0x00,
			}), // head -c 32 wikiproxy
			expectedEncoding: "",
			expectCompressed: false,
			expectStatus:     200,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := GzipMiddleware(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", tc.contentType)
					if tc.expectStatus != 200 {
						w.WriteHeader(tc.expectStatus)
					}
					if tc.body != "" {
						w.Write([]byte(tc.body))
					}
				}))

			req := httptest.NewRequest("GET", "/", nil)
			if tc.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tc.acceptEncoding)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			resp := rec.Result()
			defer resp.Body.Close()

			encoding := resp.Header.Get("Content-Encoding")
			if encoding != tc.expectedEncoding {
				t.Errorf("Expected Content-Encoding %q, got %q",
					tc.expectedEncoding, encoding)
			}

			if resp.StatusCode != tc.expectStatus {
				t.Errorf("Expected status code %d, got %d",
					tc.expectStatus, resp.StatusCode)
			}

			var body []byte
			var err error
			if tc.expectCompressed {
				var gzr *gzip.Reader
				gzr, err = gzip.NewReader(resp.Body)
				if err == nil {
					var buf bytes.Buffer
					_, err = io.Copy(&buf, gzr)
					gzr.Close()
					body = buf.Bytes()
				}
			} else {
				body, err = io.ReadAll(resp.Body)
			}
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			if string(body) != tc.body {
				t.Errorf("Expected body %q, got %q",
					tc.body, string(body))
			}
		})
	}
}
