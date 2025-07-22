// SPDX-License-Identifier: MIT
//
// Copyright (c) 2025 Aaron LI
//
// Implement middlewares:
// - panic recovery
// - gzip compression
//

package main

import (
	"compress/gzip"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
)

func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("***PANIC***", "error", err, "stack", debug.Stack())
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gzrw := &gzipResponseWriter{
			ResponseWriter: w,
			gzipWriter:     nil, // lazily initialized
		}
		defer func() {
			if gzrw.gzipWriter != nil {
				gzrw.gzipWriter.Close()
			}
		}()

		next.ServeHTTP(gzrw, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gzipWriter     *gzip.Writer
	headersWritten bool
	statusCode     int
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	// Defer writing until Content-Type is known
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.headersWritten {
		w.headersWritten = true

		if w.shouldCompress(b) {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Del("Content-Length") // gzip chunking
			w.gzipWriter = gzip.NewWriter(w.ResponseWriter)
		}

		if w.statusCode != 0 {
			w.ResponseWriter.WriteHeader(w.statusCode)
		}
	}

	if w.gzipWriter != nil {
		return w.gzipWriter.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func (w *gzipResponseWriter) shouldCompress(b []byte) bool {
	// Skip gzip if already encoded.
	encoding := w.Header().Get("Content-Encoding")
	if encoding != "" && encoding != "identity" {
		return false
	}

	contentType := w.Header().Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(b)
		w.Header().Set("Content-Type", contentType)
		slog.Debug("detected content-type", "content_type", contentType)
	}

	compressibleMIMEs := []string{
		"text/",
		"application/json",
		"application/javascript",
		"application/xml",
		"application/rss+xml",
		"application/atom+xml",
		"image/svg+xml",
	}
	for _, prefix := range compressibleMIMEs {
		if strings.HasPrefix(contentType, prefix) {
			slog.Debug("gzip response", "content_type", contentType)
			return true
		}
	}

	return false
}
