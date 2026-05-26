# syntax=docker/dockerfile:1.7
# Multi-stage build for the api service. Other binaries can reuse the
# builder layer via --target builder.

FROM golang:1.24-alpine AS builder
WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
ENV CGO_ENABLED=0 GOFLAGS=-trimpath

RUN go build \
      -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
      -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/api /app/api
COPY --from=builder /src/config.example.yaml /app/config.yaml

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/api"]
