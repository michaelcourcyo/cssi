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
      -o /out/cssi-server ./cmd/cssi-server

# ---- runtime stage ----
# The server needs LVM and NFS server userland on the host. Typically this
# image is run privileged with the host's /dev and /etc/exports bind-mounted.
FROM debian:stable-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
        lvm2 nfs-kernel-server \
        e2fsprogs xfsprogs \
        ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/cssi-server /usr/local/bin/cssi-server
ENTRYPOINT ["/usr/local/bin/cssi-server"]
