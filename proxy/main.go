package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	addr := envOr("WECHAT_PROXY_ADDR", ":8090")
	apiKey := os.Getenv("WECHAT_PROXY_API_KEY")
	appID := firstNonEmpty(os.Getenv("WECHAT_PEN_APPID"), os.Getenv("WECHAT_APPID"))
	secret := firstNonEmpty(os.Getenv("WECHAT_PEN_SECRET"), os.Getenv("WECHAT_SECRET"))

	// simple flag parse without external deps
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--addr", "-addr":
			if i+1 < len(args) {
				i++
				addr = args[i]
			}
		case "--api-key", "-api-key":
			if i+1 < len(args) {
				i++
				apiKey = args[i]
			}
		case "--appid", "-appid":
			if i+1 < len(args) {
				i++
				appID = args[i]
			}
		case "--secret", "-secret":
			if i+1 < len(args) {
				i++
				secret = args[i]
			}
		case "-h", "--help":
			printHelp()
			os.Exit(0)
		}
	}

	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "error: WECHAT_PROXY_API_KEY or --api-key required")
		os.Exit(1)
	}
	if appID == "" || secret == "" {
		fmt.Fprintln(os.Stderr, "error: WECHAT_PEN_APPID/SECRET (or --appid/--secret) required")
		os.Exit(1)
	}

	srv := newServer(apiKey, appID, secret)
	s := &http.Server{
		Addr:              addr,
		Handler:           srv.handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
	}
	fmt.Printf("wechat-proxy → http://0.0.0.0%s\n", normalizeAddr(addr))
	fmt.Printf("appid → %s\n", appID)
	fmt.Printf("auth  → Bearer API key on /v1/*\n")
	if err := s.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`wechat-pen-proxy — fixed-egress WeChat open-platform agent

Usage:
  wechat-pen-proxy [--addr :8090] [--api-key KEY] [--appid ID] [--secret SEC]

Env:
  WECHAT_PROXY_API_KEY   required
  WECHAT_PEN_APPID       WeChat AppID
  WECHAT_PEN_SECRET      WeChat AppSecret
  WECHAT_PROXY_ADDR      listen address (default :8090)

Endpoints:
  GET  /healthz
  GET  /v1/status          (auth)
  POST /v1/forward?path=/cgi-bin/...&query=...  (auth)`)
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}

func normalizeAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return addr
	}
	return ":" + addr
}
