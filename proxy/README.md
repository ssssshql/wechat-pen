# wechat-pen-proxy
#
# Independent fixed-egress agent for WeChat open-platform APIs.
# Deploy on a VPS, whitelist that IP once, point local wechat-pen at it.
#
# Quick start (compose from repo root):
#   cp .env.example .env
#   docker compose up -d --build
#
# Or build this module alone:
#   cd proxy && go build -o wechat-pen-proxy .
#   ./wechat-pen-proxy --api-key wpk_xxx --appid wx... --secret ...
#
# Endpoints:
#   GET  /healthz
#   GET  /v1/status                         Authorization: Bearer <key>
#   *    /v1/forward?path=/cgi-bin/...      Authorization: Bearer <key>
#
# Local pen config (~/.wechat-pen.json):
#   "mode": "proxy",
#   "proxy_base_url": "https://your-vps:8090",
#   "proxy_api_key": "wpk_xxx"
