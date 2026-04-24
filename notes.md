# notes

Running scratchpad — things I'd change, things I noticed, things to write up properly later.

## Proper queue + independent workers

Today the worker is one process with five goroutines (one per pipeline queue). `rdb.List().PopFront` is atomic on SQLite so you can actually scale the worker replicas right now — each event goes to exactly one instance — but a few things are still missing before I'd call it a real job queue:

- **Visibility timeout / ack:** `PopFront` is destructive. If a worker crashes mid-build, the event is gone. Only in-process panics get retried by `requeueOrFail`. A real queue (Redis Streams, NATS JetStream, SQS) redelivers after N seconds if the handler hasn't acked.
- **Real DLQ:** `pipeline.failed` is a DLQ by accident right now. A dedicated `pipeline.dlq` carrying the original event + failure history would be cleaner for postmortems.
- **Per-queue concurrency limits:** ~~`build` is expensive, `route` is ~50ms of HTTP. I want to say "2 builds concurrent, 20 routes concurrent." Goroutine-per-queue has no knob for this.~~ Now a pool of N competing goroutines per queue, sized per-queue and tunable via env (`BUILD_CONCURRENCY`, `ROUTE_CONCURRENCY`, etc.). Two uploads build in parallel now. Doesn't close the cross-process gap — a real queue with ACKs is still the right long-term answer.
- **Depth + age metrics:** No way to see "15 events pending, oldest is 30s old" — which is how you'd notice a stuck pipeline.

**Rough direction:** swap redka lists for Redis Streams (`XADD` / `XREADGROUP` gives consumer groups + acks for free) or lean on Nomad's native batch dispatch since that's where we'd run this in prod anyway.

## Startup sync scope

Sync-on-boot reconciles Caddy routes with the `deployments` table but doesn't recover in-flight queue events. Redka's `PopFront` is destructive, so a worker crash mid-build still loses that event — same visibility-timeout gap above would fix both.

Container liveness is verified via `docker inspect` before restoring a route, but the inverse isn't yet handled: a deployment row marked `running` whose container has died stays `running`. A follow-up health-check pass should either restart or transition the row to `failed`.

## Image retention

`pipeline.delete` force-removes the container and the Caddy route but doesn't touch the Docker image — rollback depends on the image still existing. Long-term that means worker-host storage grows unbounded. A cleanup job that pushes images to a local registry before `docker rmi` would cap disk use without breaking rollback.

## Log streaming — cursor + notify hybrid

Initial replay is cursor-based so reconnecting clients never lose lines. After the replay, the handler blocks on an in-process notify channel woken by the worker (HTTP POST to `/internal/notify/logs/:id`). A 30s ceiling wake runs as a safety net for a dropped notification — in the steady state this is event-driven, not polled. The hybrid avoids both the "first load is empty" failure mode of pure pub/sub and the CPU waste of pure polling.

Pipe scanning splits on `\r` and `\n` so tools that use carriage-return to overwrite the terminal line (git clone progress, railpack spinner, buildkit pulls) produce discrete log rows instead of one concatenated blob. Bursts get coalesced to ~3 visible updates/sec per stream with a guaranteed final-line flush on EOF.

## Log store

SQLite is pulling double duty here: OLTP-shaped engine handling append-heavy, time-ordered log data. It works because volumes are small and reads are cursor-paginated, but it's not the right long-term shape. 

A proper log backend (Loki for cheap object-storage + label indexing, ClickHouse if we ever want fast aggregation queries) would scale cleanly, give us real search, and decouple log write amplification from the deployment table.

## Brimble deploy

Plan: deploy my knowledge API — actually want to use the $5 plan for real, not a throwaway hello-world. Angle for the feedback write-up: "paid for the $5 plan because I wanted this deployed somewhere I'd actually use, planning to move more of my side projects over."

Things to track while deploying so I have honest, specific feedback:

- Signup / onboarding friction.
- First deploy flow (where did I get stuck, what error messages were unhelpful).
- Build log experience vs what I built here.
- Custom domain setup.
- Anything I hit that required reaching out to @pipe_dev on Twitter.
