FROM golang:1.25.0-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
COPY vendor ./vendor
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -mod=vendor -o /worker ./cmd/worker

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y \
    ca-certificates git curl sqlite3 \
    && rm -rf /var/lib/apt/lists/*

# docker CLI: used as a fallback by railpack / available for local debugging.
# BuildKit does the actual build work via BUILDKIT_HOST.
RUN curl -fsSL https://get.docker.com | sh

# railpack CLI: builds source into images via buildkitd.
RUN curl -sSL https://railpack.com/install.sh | sh

COPY --from=builder /worker /worker
CMD ["/worker"]
