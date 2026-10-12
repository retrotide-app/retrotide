ARG APP_VERSION=dev

FROM node:lts-alpine AS frontend

ARG APP_VERSION

ENV VITE_APP_VERSION=${APP_VERSION}

WORKDIR /src/frontend

COPY frontend/package.json frontend/package-lock.json ./

RUN --mount=type=cache,target=/root/.npm npm ci

COPY frontend/ ./

RUN npm run build

FROM golang:1.27-alpine AS builder

ARG APP_VERSION

ENV CGO_ENABLED=0

WORKDIR /src

COPY . .

COPY --from=frontend /src/static ./static

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-X retrotide/internal/config.Version=${APP_VERSION}" -o /out/retrotide .

FROM alpine:latest

RUN apk add --no-cache libwebp-tools

RUN addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app -s /sbin/nologin app

WORKDIR /app

COPY --from=builder /out/retrotide ./retrotide

USER 10001:10001

EXPOSE 4323

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --start-interval=2s --retries=3 \
    CMD ["wget", "-q", "-O", "/dev/null", "http://127.0.0.1:4323/livez"]

CMD ["./retrotide"]
