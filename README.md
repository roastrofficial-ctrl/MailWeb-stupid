# MailWeb

MailWeb is an experiment in transporting web-style documents as private messages instead of serving them directly as HTTP responses. A page still feels like a normal website to the person browsing it, but each navigation is represented internally by a request message and a response message.

This repository proves one complete round trip and deliberately does little else:

```text
Client -> message transport -> Laravel publisher -> message response -> client renderer
terminal   Go postbox          demo-site          JSON document       Go postbox
```

MailWeb messages are independent of their carrier. This repository now provides two development transports behind the same interface: a direct synchronous HTTP exchange and an SMTP exchange through Mailpit. The application contract remains only the request and response documented in [`packages/protocol`](packages/protocol).

## Repository layout

- `apps/postbox` — Go graphical and terminal clients, transports, protocol validator, and renderers
- `apps/demo-site` — Laravel document publisher
- `packages/protocol` — protocol notes, JSON Schema, and examples
- `packages/laravel-mailweb` — placeholder for a future Laravel integration package
- `extension` — placeholder for a future browser extension; it is intentionally not implemented

## Browse locally

Docker and Docker Compose are the only host requirements.

### Graphical Postbox

Start the graphical browser with SMTP as its carrier:

```bash
./postbox-ui
```

Open <http://127.0.0.1:9847>, enter `mailweb://demo.local/`, and press **Send**. Every graphical navigation travels out and back through Mailpit. The browser includes back, forward, reload, safe structured document rendering, and a message inspector showing the exact request, response, timing, status, and transport.

Set `MAILWEB_UI_TRANSPORT=http` when launching to use the direct development carrier instead.

### Terminal Postbox

Launch the terminal postbox with a MailWeb URI:

```bash
./postbox mailweb://demo.local/
```

Enter a displayed number to navigate between the demo documents, or `q` to quit. The launcher starts the Laravel publisher automatically. To inspect the publisher directly:

```bash
curl -s http://localhost:8081/mailweb/messages \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @packages/protocol/examples/request.json
```

The command above uses the original direct HTTP development transport. To make every navigation travel out and back as actual email through SMTP:

```bash
./postbox --transport smtp mailweb://demo.local/
```

Postbox prints `Waiting for the publisher to reply...` while polling its generated mailbox. Browse the underlying request and response emails in Mailpit at <http://localhost:8025>. The default reply timeout is 15 seconds and can be changed with `--timeout`, for example `--timeout 30s`.

The publisher's HTTP port is exposed only as a local debugging convenience. Normal browsing goes through the terminal postbox.

Both carriers implement the small Go `Transport` interface in `apps/postbox/cmd/postbox/transport.go`. HTTP endpoints, SMTP addresses, MIME headers, and Mailpit polling never appear in MailWeb request or response objects.

## Local development

Compose bind-mounts `apps/postbox` at `/app` in the Go container and `apps/demo-site` at `/app` in the Laravel container. Each `./postbox` invocation compiles the current Go source. Laravel reads PHP files on each request.

Laravel's generated `vendor` directory is kept in the Docker-managed `demo-site-vendor` volume instead of being written into the repository.

## Scope

There is intentionally no database, queue, event bus, cache, authentication, encryption, service discovery, or orchestration beyond Compose. Those concerns should only be introduced after the basic request-message/response-message model has proved useful.
