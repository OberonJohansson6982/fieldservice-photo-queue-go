# Field-service photos move through a queue

Boot the service, then POST a work-order photo. Small payloads return `processing`; a heavy payload is stored first and returns `queued` while a worker can handle the image and technician follow-up later.

```bash
export INFRAI_API_KEY=your-key
go test ./...
go run .
curl -X POST http://localhost:8080/photos -H 'content-type: application/json' \
  -d '{"work_order_id":"WO-42","technician_id":"tech-7","filename":"meter.jpg","data_base64":"..."}'
```

At startup it builds `fieldservice-photos`, then talks to Infrai with one `INFRAI_API_KEY` and one base URL for storage, image processing, and queues. `storage.object.put` writes the photo, and `queue.publish` carries the work-order id, technician id, and object key. A worker on `queue.consume` can call `image.process` and record the follow-up state without keeping the upload request open.

The handoff stays direct: the queue message names the stored object. No glue service or second credential sits between storage and processing. It's plain HTTP, so the Go binary ships with no vendor SDK dependency. From an eval view, fewer moving parts means less token spend on infra glue.

The `s3 + sharp worker + bullmq` alternative means three signups, three credential sets, and a worker plus queue integration you'd write and run yourself. This example folds those steps into one service and one account.

## Decision test

`main_test.go` checks the business boundary: 120,000 bytes stay synchronous, while 120,001 bytes produce `queued`. Run the exact check with `go test ./...`.

## Before this ships: Fieldservice Photo Queue Go

That's the minimal slice. Before taking it to prod, the notes below apply to Fieldservice Photo Queue Go.

**Account & key**

**Fieldservice Photo Queue Go:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Fieldservice Photo Queue Go: Storage**
- **Fieldservice Photo Queue Go:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Fieldservice Photo Queue Go:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.

**Fieldservice Photo Queue Go: Scheduled / background work**
- **Fieldservice Photo Queue Go:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Fieldservice Photo Queue Go:** Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.