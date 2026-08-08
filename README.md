# MailWeb

MailWeb is an experiment in transporting web-style documents as private messages instead of serving them directly as HTTP responses. A page still feels like a normal website to the person browsing it, but each navigation is represented internally by a request message and a response message.

This repository proves one complete round trip and deliberately does little else:

```text
Client -> message transport -> Laravel publisher -> message response -> client renderer
browser    Go postbox          demo-site          JSON envelope       Go postbox
```

For this first experiment, the message transport is a synchronous HTTP POST carrying JSON between two local containers. HTTP is only the local carrier; the application contract is the message envelope documented in [`packages/protocol`](packages/protocol). The postbox is the only service exposed to the browser, and the Laravel application acts as the document publisher.

## Repository layout

- `apps/postbox` — Go client gateway, message transport, and renderer
- `apps/demo-site` — Laravel document publisher
- `packages/protocol` — protocol notes, JSON Schema, and examples
- `packages/laravel-mailweb` — placeholder for a future Laravel integration package
- `extension` — placeholder for a future browser extension; it is intentionally not implemented

## Run locally

Docker and Docker Compose are the only host requirements.

```bash
docker compose up --build
```

Open <http://localhost:8080>. The rendered page includes its MailWeb message ID, making the round trip visible. To inspect the publisher directly:

```bash
curl -s http://localhost:8081/mailweb/messages \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @packages/protocol/examples/request.json
```

The publisher's port is exposed only as a local debugging convenience. Normal browsing goes through postbox on port `8080`.

## Scope

There is intentionally no database, queue, event bus, cache, authentication, encryption, service discovery, or orchestration beyond Compose. Those concerns should only be introduced after the basic request-message/response-message model has proved useful.
