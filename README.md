# not_brimble

A one-page deployment pipeline: submit a Git URL or upload a tarball, get a container image built by Railpack, a running container, and a Caddy fronted URL all driven by a single API and a worker that owns every side effect.

## Architecture and Decisions

This project follows an Event-Driven Architecture (EDA) to handle asynchronous container builds and deployments. For a detailed breakdown of the technical decisions, state management with Redka, and reliability patterns, please read:

- [ARCHITECTURE.md](./architecture.md)

Additionally, feedback on the Brimble platform deployment experience can be found here:

- [FEEDBACK](./feedback/README.md)

## TL;DR

```bash
docker compose up --build
# UI/Caddy:  http://localhost:80   (deployments reachable at http://<id>.localhost)
# API/Docs:  http://localhost/api/docs
```

No accounts, no external services, no config. First run pulls the Railpack and BuildKit images; subsequent runs are fast.

## Architecture

```
┌────────┐  POST /deployments     ┌─────────┐
│  UI    │ ─────────────────────► │   API   │
│ (Vite) │ ◄───── SSE ──────────  │  (Gin)  │
└────────┘                        └────┬────┘
                                       │ redka list (queue)
                                       ▼
                                 ┌───────────┐   ┌───────────┐
                                 │  Worker   │──►│ buildkitd │ (Railpack build)
                                 │ (4 stages)│   └───────────┘
                                 └─────┬─────┘
                                       │ docker.sock (run), caddy admin (route)
                                       ▼
                                 ┌───────────┐
                                 │   Caddy   │ :80 → deploy-<name>:3000
                                 └───────────┘
```

Two processes, event-driven between them:

- **API** (`cmd/api`) is a thin control plane. Creates rows, enqueues events, serves SSE streams. It never touches Docker or Caddy.
- **Worker** (`cmd/worker`) owns all infrastructure side effects: build, run, route, delete, failure cleanup. One consumer goroutine per pipeline queue.

State lives in SQLite (deployment rows + log lines). Queues live in [redka](https://github.com/nalgeon/redka) (Redis-compatible, backed by the same SQLite file).

### Pipeline stages

| Queue | Handler | Transitions |
| :--- | :--- | :--- |
| `pipeline.queued` | `BuildHandler` | pending → building → built |
| `pipeline.built` | `RunHandler` | built → deploying |
| `pipeline.running` | `RouteHandler` | deploying → running |
| `pipeline.failed` | `FailureHandler` | anything → failed |
| `pipeline.delete` | `DeleteHandler` | anything → stopped |

Each handler reads/writes the deployment row and emits log lines; the SSE endpoints read back from SQLite directly.

### Log streaming

Logs are persisted to SQLite as the worker writes them. SSE clients stream them via a cursor on `log_lines.id` (`GET /deployments/:id/logs`):

- **Multi-viewer safe** — each client keeps its own cursor.
- **Reconnect-safe** — reconnecting clients pick up where they left off.
- **Scroll-back** — just the first page of the query.

The stream closes once the deployment reaches a terminal status (`running`, `failed`, `stopped`) and all preceding log lines have been delivered.

### Caddy routing

Each deployment gets a stable `@id` tag in Caddy (`deploy-<name>`, where `<name>` is the DNS-safe slug built from the source) so that later DELETEs target the route directly — no index-based mutation games. Caddy and every deployment container share the `not_brimble_net` bridge network; containers are reachable by name, so no host port bindings are needed (and none are published). 

On worker startup the route table is reconciled against the DB (gated on `docker inspect`) so a Caddy or worker restart doesn't orphan live URLs.

### Build cache

`buildkitd`'s `/var/lib/buildkit` is a named volume, so layer cache (npm install, apt install, runtime downloads) survives `docker compose down` and is reused on the next build of the same repo. First deploy pays the full cost; subsequent deploys reuse unchanged layers.

## API

```
POST   /deployments              # create (JSON or multipart)
GET    /deployments              # list
GET    /deployments/:id          # detail
DELETE /deployments/:id          # enqueue delete (202 Accepted)

GET    /deployments/:id/logs     # SSE; replay + live
GET    /deployments/:id/status   # SSE; status changes only
GET    /health
```

### Create — JSON

```bash
curl -X POST http://localhost:8080/deployments \
  -H 'Content-Type: application/json' \
  -d '{"source_type":"git","source_url":"https://github.com/railwayapp-templates/node-express"}'
```

### Create — upload

```bash
tar czf /tmp/app.tar.gz -C ./my-app .
curl -X POST http://localhost:8080/deployments \
  -F source_type=upload \
  -F file=@/tmp/app.tar.gz
```

The deployment is returned immediately with `status=pending`; watch the SSE streams for the rest of the lifecycle.

## Stack decisions

| Choice | Why |
| :--- | :--- |
| **Go** | First-class Docker SDK; SSE needs no library; redka is Go-native |
| **SQLite + WAL** | One file, handles the load, no extra service |
| **redka for queues** | Redis semantics on the same SQLite file — free with the DB |
| **Railpack + buildkitd** | Required by Railpack; runs as a dedicated service over TCP |
| **ULIDs** | Time-sortable IDs without a `created_at` index; lower-cased where Docker/Caddy require |
| **Stable Caddy `@id`** | Position-free deletes; idempotent re-adds possible later |
| **Worker owns side effects** | API has no Docker socket mount — smaller blast radius |
| **Two SQLite pools** | Deliberate separation of concerns (api + worker isolated) |
| **Notify broker over HTTP** | Redka has no Pub/Sub; used for worker → API log wakes |

## What I'd do with another weekend

- **Proper job queue:** Swap Redka for Redis Streams or NATS JetStream to get visibility timeouts, ACKs, and per-queue concurrency knobs.
- **Image retention via registry:** Push built images to a local registry and `docker rmi` old versions to prevent unbounded disk growth.
- **Dynamic service port:** Detect the port from Railpack/EXPOSE instead of hardcoding `PORT=3000`.
- **Proper secrets management:** Add per-service scoped secrets, rotation, and log masking using a secrets backend.
- **Zero-downtime redeploy:** Implement a true blue/green swap by health-gating the new container before cutting traffic in Caddy.
- **Proper health gate:** Poll HTTP/TCP checks before marking a deployment as `running`.
- **Terminal vs transient error classification:** Use sentinel error types to differentiate between auth failures (fail fast) and network blips (retry).
- **Log store built for logging:** Move from SQLite to a purpose-built log backend (Loki/ClickHouse) for better scaling and search.
- **Parallel image warmup:** Kick off base image pulls during the `git clone` phase to hide first-deploy latency.

## What I'd rip out

- **In-container Docker CLI:** Swap the `curl | sh` install for a proper `docker-ce-cli` package.
- **CORS wildcard:** Scope the CORS policy properly before moving beyond local development.

## Repo layout

```text
cmd/
  api/          HTTP server (Gin) + SSE streams
  worker/       queue consumers + handler wiring
internal/
  db/           SQLite schema, deployment + log line queries
  events/       redka-backed queue + event types
  pipeline/     build, run, route, failure, delete handlers
  docker/       thin Docker SDK wrapper used by worker
  caddy/        admin API client (add/delete routes by @id)
caddy/
  config.json   Caddy bootstrap (admin API + :80 srv0)
docker-compose.yml
```

## Tests

```bash
go test ./...
```

Coverage focuses on critical paths:
- `internal/pipeline/build_test.go`: Git clone retry and tar extraction.
- `internal/caddy/client_test.go`: Route addition/deletion round-trips.
- `cmd/worker/main_test.go`: Panic recovery and retry backoff logic.
- `internal/db/db_test.go`: CRUD operations and log cursor pagination.

## Time spent

Roughly one focused day on backend + infra. Frontend is a separate pass.

## Brimble deploy + feedback

Deployed a non-trivial project ([jigsaw.brimble.app](https://jigsaw.brimble.app)) on the Hacker plan. Hit one hard blocker regarding quotas which was cleared same-day. Full write-up in [`feedback/`](./feedback/README.md).
