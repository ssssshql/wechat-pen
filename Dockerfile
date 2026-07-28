# wechat-pen — full app with Chromium (QR login / CDP whitelist / future publish)
# Build:  docker build -t wechat-pen .
# Run:    docker run --rm -p 8080:8080 --shm-size=1g -v wechat-pen-data:/data wechat-pen
#
# Headless Chromium is required inside the container (no display).
# QR is scraped and sent to the browser UI — users still scan with WeChat on phone.

# ---- frontend ----
FROM node:22-bookworm AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- backend ----
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN rm -rf server/spa && mkdir -p server/spa
COPY --from=web /web/dist/ server/spa/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/wechat-pen .

# ---- runtime (Chromium + fonts for QR / CDP) ----
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      chromium \
      fonts-noto-cjk \
      fonts-liberation \
      ca-certificates \
      dumb-init \
    && rm -rf /var/lib/apt/lists/* \
    && chromium --version

ENV HOME=/data \
    WECHAT_PEN_HEADLESS=1 \
    CHROME_PATH=/usr/bin/chromium \
    # Chromium in Docker
    CHROMIUM_FLAGS="--no-sandbox --disable-gpu --disable-dev-shm-usage"

WORKDIR /
COPY --from=build /out/wechat-pen /usr/local/bin/wechat-pen

RUN useradd -m -u 10001 -s /usr/sbin/nologin pen \
    && mkdir -p /data \
    && chown -R pen:pen /data

USER pen
VOLUME ["/data"]
EXPOSE 8080
# dumb-init reaps zombie chromium processes
ENTRYPOINT ["dumb-init", "--"]
CMD ["wechat-pen", "serve", "--addr", ":8080"]
