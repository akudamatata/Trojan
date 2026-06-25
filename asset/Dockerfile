# Stage 1: Build the Frontend (Vue 3.0 Web UI)
FROM --platform=$BUILDPLATFORM node:16-alpine AS web-builder

WORKDIR /web

# Clone the original frontend repository
RUN apk add --no-cache git && \
    git clone https://github.com/akudamatata/trojan-web.git .

# Install dependencies and build the static files
RUN npm install && \
    npm run build

# Stage 2: Build the Go binary
FROM --platform=$BUILDPLATFORM golang:1.21-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Copy the compiled web UI files from web-builder to web/templates
COPY --from=web-builder /web/dist ./web/templates

# Build the binary for the target architecture
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags "-w -s" -o trojan .

# Stage 3: Final image (debian bookworm-slim, Debian 12 stable)
FROM --platform=$TARGETPLATFORM debian:bookworm-slim

# Copy the pre-built binary from the builder stage
COPY --from=builder /app/trojan /usr/local/bin/trojan

# Copy the systemd service file directly from source code
COPY asset/trojan-web.service /etc/systemd/system/trojan-web.service

ARG SYSTEMCTL_URL="https://raw.githubusercontent.com/gdraheim/docker-systemctl-replacement/master/files/docker/systemctl.py"

RUN for i in 1 2 3 4 5; do apt-get update && break || (echo "apt-get update failed, retry $i..." && sleep 15); done && \
    apt-get install -y --no-install-recommends \
        socat \
        bash-completion \
        cron \
        iptables \
        openssl \
        unzip \
        curl \
        ca-certificates && \
    curl -L --retry 5 --retry-delay 3 $SYSTEMCTL_URL -o /usr/bin/systemctl && \
    chmod +x /usr/local/bin/trojan /usr/bin/systemctl && \
    echo "source <(trojan completion bash)" >> ~/.bashrc && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

