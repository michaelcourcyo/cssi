# syntax=docker/dockerfile:1.6

# ---- build stage ----
FROM golang:1.22 AS build
ARG VERSION=dev
ARG COMMIT=none
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath \
      -ldflags "-s -w -X github.com/michaelcourcyo/cssi/internal/version.Version=${VERSION} -X github.com/michaelcourcyo/cssi/internal/version.Commit=${COMMIT}" \
      -o /out/cssi-driver ./cmd/cssi-driver

# ---- runtime stage ----
# The node plugin needs NFS client tools to mount exports.
FROM debian:stable-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends nfs-common ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/cssi-driver /usr/local/bin/cssi-driver
ENTRYPOINT ["/usr/local/bin/cssi-driver"]
