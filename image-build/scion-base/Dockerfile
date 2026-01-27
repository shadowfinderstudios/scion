# syntax=docker/dockerfile:1
# Scion Base Image - Adds sciontool and scion user on top of core-base
ARG BASE_IMAGE=us-central1-docker.pkg.dev/ptone-misc/public-docker/core-base:latest
FROM ${BASE_IMAGE} AS base

# Builder stage for sciontool compilation
FROM base AS builder
USER root
WORKDIR /src
ENV PATH="/usr/local/go/bin:${PATH}"
ENV CGO_ENABLED=1

# Copy go.mod and go.sum first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build scion and sciontool
COPY cmd/ ./cmd/
COPY pkg/ ./pkg/
RUN go build -ldflags="-s -w" -o /usr/local/bin/scion ./cmd/scion/ && \
    go build -ldflags="-s -w" -o /usr/local/bin/sciontool ./cmd/sciontool/

# Final stage
FROM base
USER root

# Copy scion and sciontool from builder
COPY --from=builder /usr/local/bin/scion /usr/local/bin/scion
COPY --from=builder /usr/local/bin/sciontool /usr/local/bin/sciontool

# Create scion user with passwordless sudo
RUN useradd -m -s /bin/zsh -u 1001 scion && \
    echo "scion ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/scion && \
    chmod 0440 /etc/sudoers.d/scion

# Set ownership for npm global directory
RUN chown -R scion:scion /usr/local/share/npm-global

USER scion

# Set sciontool as the container entrypoint (PID 1)
# The -- separator ensures everything after it is treated as the child command
ENTRYPOINT ["sciontool", "init", "--"]

# Default command (can be overridden at runtime)
CMD ["bash"]
