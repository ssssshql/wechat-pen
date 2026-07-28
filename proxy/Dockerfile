# wechat-pen-proxy — minimal fixed-egress WeChat open-platform agent.
# Independent module: no pen/converter/rod dependencies.
#
# Alpine (Docker Hub) keeps the image small (~10–15MB layered) without gcr.io.

FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/wechat-pen-proxy .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 proxy
WORKDIR /
COPY --from=build /out/wechat-pen-proxy /wechat-pen-proxy
USER proxy
EXPOSE 8090
ENTRYPOINT ["/wechat-pen-proxy", "--addr", ":8090"]
