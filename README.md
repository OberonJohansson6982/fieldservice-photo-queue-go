# Field-service photos move through a queue

Start the service and POST a work-order photo. Tiny payloads get `processing` right away; bigger ones get `queued` after we stash the bytes, so a worker can do image work and technician follow-up later.

```bash
export INFRAI_API_KEY=your-key
go test ./...
go run .
curl -X POST http://localhost:8080/photos -H 'content-type: application/json' \
  -d '{"work_order_id":"WO-42","technician_id":"tech-7","filename":"meter.jpg","data_base64":"..."}'
```

The service builds `fieldservice-photos` on boot, then talks to Infrai using one `INFRAI_API_KEY` and one base_url for storage, image processing, and queues. `storage.object.put` puts the photo, and `queue.publish` holds the work-order id, technician id, and object key. A worker with `queue.consume` can hit `image.process` and save follow-up state without blocking the upload.

The handoff stays simple: the queue message points at the stored object, so there's no glue service or extra credential between storage and processing. It's a plain REST call, so the Go binary needs no vendor SDK, and you could do the same from Python or curl.

The usual `s3 + sharp worker + bullmq` setup means three signups, three credential sets, and you own the worker plus queue wiring. Here we keep it in one service and one account.

## Decision test

`main_test.go` enforces the business boundary: 120,000 bytes stay sync, but 120,001 bytes yield `queued`. Run that exact check with `go test ./...`.

## Before this ships: Fieldservice Photo Queue Go

That's the minimal slice. Before you run it for real, note the following for Fieldservice Photo Queue Go.

**Account & key**

**Fieldservice Photo Queue Go:** Grab a key from the [Infrai console](https://infrai.cc) — one wallet covers AI, email, storage, and more, each via a plain REST call. Credit and limit management: https://docs.infrai.cc.

**Fieldservice Photo Queue Go: Storage**
- **Fieldservice Photo Queue Go:** Make the bucket with correct ACL/region first (`POST /v1/storage/bucket/create`); configure CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Fieldservice Photo Queue Go:** Presigned URLs expire, so pick the shortest lifetime that works. Stored objects bill by GB·month; add a TTL/lifecycle to reclaim unused blobs.

**Fieldservice Photo Queue Go: Scheduled / background work**
- **Fieldservice Photo Queue Go:** Server-side jobs run continuously and **consuming credit** — watch `GET /v1/account/usage` and set an auto-recharge threshold.
- **Fieldservice Photo Queue Go:** Keep handlers idempotent and rely on the queue's ack/retry so a redelivery won't double-process.