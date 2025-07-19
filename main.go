// SPDX-License-Identifier: MIT
//
// Wikiproxy - Simple Wikipedia proxy
//

package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	// Logger level: debug, info, warn, error
	LogLevel string `toml:"log_level"`
	// Listen address (host:port)
	// e.g., 127.0.0.1:2012
	Listen string `toml:"listen"`
	// The proxy used for accessing Wikipedia
	// e.g., socks5h://127.0.0.1:1080
	Proxy string `toml:"proxy"`
	// The domains used for proxying Wikipedia
	Domains struct {
		// Domain to proxy the English site: en.wikipedia.org
		English string `toml:"english"`
		// Domain to proxy the Chinese site: zh.wikipedia.org
		Chinese string `toml:"chinese"`
	} `toml:"domains"`
}

func main() {
	logLevel := &slog.LevelVar{} // INFO
	logOpts := &slog.HandlerOptions{
		AddSource: true,
		Level:     logLevel,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, logOpts))
	slog.SetDefault(logger)

	configFile := flag.String("config", "wikiproxy.toml", "configuration file")
	isDebug := flag.Bool("debug", false, "debug mode")
	flag.Parse()

	if *isDebug {
		logLevel.Set(slog.LevelDebug)
	}

	config := Config{}
	if _, err := toml.DecodeFile(*configFile, &config); err != nil {
		slog.Error("failed to read config", "file", *configFile, "error", err)
		os.Exit(1)
	}
	slog.Debug("read config", "file", *configFile, "data", config)

	if *isDebug {
		config.LogLevel = "debug"
	}
	switch config.LogLevel {
	case "", "info":
		logLevel.Set(slog.LevelInfo)
	case "debug":
		logLevel.Set(slog.LevelDebug)
	case "warn":
		logLevel.Set(slog.LevelWarn)
	case "error":
		logLevel.Set(slog.LevelError)
	default:
		slog.Warn("unknown log level", "level", config.LogLevel)
	}

	// TODO

	slog.Info("starting server", "url", "http://"+config.Listen)
	err = http.ListenAndServe(config.Listen, nil)
	if err != nil {
		panic(err)
	}
}
