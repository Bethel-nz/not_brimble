FROM golang:1.25.0-bookworm AS builder
WORKDIR /app

# Install the swag CLI outside of vendor mode. GOFLAGS="" overrides any
# inherited -mod=vendor so `go install` can fetch the binary fresh from
# the network instead of looking for it in our vendor tree (we intentionally
# don't vendor swaggo to keep the runtime binary small).
ENV GOFLAGS=""
RUN go install github.com/swaggo/swag/cmd/swag@latest

COPY go.mod go.sum ./
COPY vendor ./vendor
COPY . .

# Regenerate swagger spec from the //@ annotations in the source.
#   --outputTypes json,yaml deliberately skips the Go file (docs.go) — we
#   only need the JSON for Scalar at runtime, and omitting docs.go keeps
#   the subsequent `-mod=vendor` build from demanding a vendored swaggo.
RUN rm -f docs/docs.go && swag init -g cmd/api/main.go -o ./docs --outputTypes json,yaml

RUN CGO_ENABLED=1 GOOS=linux go build -mod=vendor -o /api ./cmd/api

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates sqlite3 && rm -rf /var/lib/apt/lists/*
COPY --from=builder /api /api
# main.go serves this at /docs/openapi.json for the Scalar UI.
COPY --from=builder /app/docs/swagger.json /docs/swagger.json
EXPOSE 8080
CMD ["/api"]
