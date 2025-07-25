// SPDX-License-Identifier: MIT
//
// Copyright (c) 2025 Aaron LI

package main

import (
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
}
