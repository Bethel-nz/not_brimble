FROM oven/bun:latest AS builder
WORKDIR /app

# Copy lockfiles and install dependencies
COPY package.json bun.lock* ./
RUN bun install --frozen-lockfile || bun install

# Copy source and build from scratch
COPY . .
RUN bun run build

# Serve with Caddy
FROM caddy:alpine
COPY --from=builder /app/dist /srv
COPY Caddyfile /etc/caddy/Caddyfile
CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile"]
