# Stage 1: Build the Go binary
FROM --platform=$BUILDPLATFORM golang:1.21-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the binary for the target architecture
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags "-w -s" -o trojan .

# Stage 2: Final image
FROM --platform=$TARGETPLATFORM centos:7

# Copy the pre-built binary from the builder stage
COPY --from=builder /app/trojan /usr/local/bin/trojan

# Copy the systemd service file directly from source code
COPY asset/trojan-web.service /etc/systemd/system/trojan-web.service

ARG SYSTEMCTL_URL="https://raw.githubusercontent.com/gdraheim/docker-systemctl-replacement/master/files/docker/systemctl.py"

RUN yum install socat bash-completion crontabs iptables openssl unzip curl -y && \
    curl -L $SYSTEMCTL_URL -o /usr/bin/systemctl && \
    chmod +x /usr/local/bin/trojan /usr/bin/systemctl && \
    echo "source <(trojan completion bash)" >> ~/.bashrc && \
    yum clean all
