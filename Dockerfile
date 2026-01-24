# Stage 1: Build the frontend
#FROM docker.io/library/node:22-alpine AS frontend-builder
FROM registry.access.redhat.com/ubi9/nodejs-20:9.7 AS frontend-builder
USER 0
WORKDIR /opt/app-root/src

# ENV RUST_BACKTRACE=full \
#     PATH="${PATH}:${PWD}/node_modules/.bin" \
#     RUST_MIN_STACK=16777216
# # Build SWC...
# RUN dnf install -y git gcc lld llvm-toolset gcc-toolset-14 jq && \
#     curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y && \
#     npm install -g corepack deno && \
#     corepack enable && \
#     echo "y" | yarn set version stable && \
#     cp $HOME/.cargo/env /etc/profile.d/cargo.sh && \
#     source /etc/profile.d/cargo.sh && \
#     curl https://rustwasm.github.io/wasm-pack/installer/init.sh -sSf | sh

# RUN scl enable gcc-toolset-14 bash && \
#     ln -s /usr/bin/aarch64-redhat-linux-gcc /usr/bin/aarch64-linux-gnu-gcc && \
#     source /etc/profile.d/cargo.sh && \
#     git clone https://github.com/swc-project/swc.git && \
#     cd swc && \
#     git checkout v1.15.10 && \
#     git submodule update --init --recursive && \
#     rustup target add wasm32-wasip1 && \
#     yarn && \
#     yarn dlx jest@27 mocha && \
#     cargo test --all --no-default-features --features swc_v2 --features filesystem_cache

# Copy frontend package files
#COPY frontend/package.json ./

# Install dependencies
#RUN npm ci

# Copy frontend source
COPY frontend/ ./

# Build static export
RUN npm install --force && npm ci && npm run build

# Stage 2: Build the backend
#FROM docker.io/library/golang:1.25-alpine AS backend-builder
FROM registry.access.redhat.com/ubi8/go-toolset:1.25 AS backend-builder

USER 0

# Install build dependencies
RUN dnf install -y git ca-certificates

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Copy frontend build from previous stage
COPY --from=frontend-builder /opt/app-root/src/out ./frontend/out

# Prepare embedded frontend
RUN rm -rf web/static && \
    mkdir -p web/static && \
    cp -r frontend/out/* web/static/

# Build the binary with version info
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT=unknown

#RUN CGO_ENABLED=0 GOOS=linux go build \
RUN GOOS=linux go build \
    -ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT} -s -w" \
    -o /app/bin/msaki \
    ./cmd/msaki

# Stage 3: Final minimal image
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

# Labels for OpenShift/Kubernetes
LABEL name="msaki" \
      summary="Model Swiss Army Knife Interface" \
      description="Unified interface for managing and proxying AI model endpoints" \
      maintainer="Ken Moini" \
      version="1.0" \
      io.k8s.display-name="MSAKI" \
      io.k8s.description="Model Swiss Army Knife Interface - Proxy and manage AI model endpoints" \
      io.openshift.tags="ai,llm,proxy,openai,ollama"

# Install ca-certificates for HTTPS requests to external APIs
RUN microdnf install -y ca-certificates && microdnf update -y && \
    microdnf clean all && \
    rm -rf /var/cache/yum

# Create non-root user
RUN groupadd -r -g 1001 msaki && useradd -r -g msaki -u 1001 msaki

# Create directories for configs and logs
RUN mkdir -p /etc/msaki /var/log/msaki && \
    chown -R msaki:msaki /etc/msaki /var/log/msaki

# Copy binary from builder
COPY --from=backend-builder /app/bin/msaki /usr/local/bin/msaki

# Use non-root user
USER msaki

# Default config location
WORKDIR /etc/msaki

# Expose default port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/msaki", "-health-check"] || exit 1

# Default command
ENTRYPOINT ["/usr/local/bin/msaki"]
CMD ["-config", "/etc/msaki/msaki.yaml"]
