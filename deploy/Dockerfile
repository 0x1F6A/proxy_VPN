# syntax=docker/dockerfile:1.7
# Multi-stage build that produces a single binary based on the BIN build
# arg. Build any of the cmd/ entrypoints:
#
#   docker build --build-arg BIN=api         -t proxy-vpn/api .
#   docker build --build-arg BIN=worker      -t proxy-vpn/worker .
#   docker build --build-arg BIN=node-agent  -t proxy-vpn/node-agent .
#   docker build --build-arg BIN=usdt-watcher -t proxy-vpn/usdt-watcher .
#   docker build --build-arg BIN=admin       -t proxy-vpn/admin .

FROM golang:1.25-alpine AS builder
WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG BIN=api
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
ENV CGO_ENABLED=0 GOFLAGS=-trimpath

RUN go build \
      -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
      -o /out/app ./cmd/${BIN}

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/app /app/app
COPY --from=builder /src/config.example.yaml /app/config.yaml

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/app"]
