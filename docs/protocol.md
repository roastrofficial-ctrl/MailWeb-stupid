# MailWeb Protocol v0.3

## Status

This document defines the experimental MailWeb Protocol v0.3. The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** carry their usual RFC meanings.

## 1. Scope and transport independence

MailWeb describes application requests, responses, and safe semantic documents. It does not describe delivery. Identical JSON may travel through SMTP, HTTP, files, WebSockets, or another carrier. Carrier addresses, framing, retries, encryption, and connection state MUST NOT appear as protocol-specific fields.

Messages are UTF-8 JSON objects. The canonical [request](../packages/protocol/schema/mailweb-request.schema.json), [response](../packages/protocol/schema/mailweb-response.schema.json), and [document](../packages/protocol/schema/mailweb-document.schema.json) schemas are closed; unknown fields are invalid.

## 2. Requests

Every request contains `mailweb: "0.3"`, a fresh uppercase ULID `id`, `method`, an absolute `mailweb://` URI, and string-valued `headers`.

### 2.1 GET and query parameters

GET retrieves a document and MUST NOT contain `body`. Query parameters belong in the URI and use normal URI percent-encoding:

```json
{
  "mailweb": "0.3",
  "id": "01J00000000000000000000000",
  "method": "GET",
  "uri": "mailweb://demo.local/search?q=internet",
  "headers": {}
}
```

The query is part of resource identity. Publishers parse it from `uri`; no parallel query structure exists.

### 2.2 POST and request bodies

POST submits application data. In v0.3, `body` is a JSON object and is REQUIRED for POST. A JSON body SHOULD be described by `content-type: application/json`:

```json
{
  "mailweb": "0.3",
  "id": "01J00000000000000000000000",
  "method": "POST",
  "uri": "mailweb://demo.local/hello",
  "headers": {"content-type": "application/json"},
  "body": {"name": "Levi"}
}
```

Headers are MailWeb application metadata, not SMTP or HTTP headers. v0.3 defines no cookies, sessions, or authentication.

## 3. Responses and correlation

A response contains `mailweb: "0.3"`, `request_id`, an integer `status` from 100 through 599, and a `document`. `request_id` MUST exactly match the request `id`. Status describes the MailWeb result, not carrier delivery. A document is required even for errors.

## 4. Documents

A document contains a plain-text `title` and ordered `body`. Text is never markup. Renderers MUST escape it and MUST NOT interpret HTML, Markdown, JavaScript, templates, or event handlers.

v0.3 supports:

- `heading`: `level` 1–6 and plain `text`.
- `paragraph`: plain `text`.
- `link`: plain `label` and URI-reference `href`.
- `image`: URI-reference `src` and plain `alt`.
- `button`: navigational `label` and `href`; it executes no code and submits no data.
- `form`: semantic input and submission metadata described below.

References resolve against the request URI. Clients MUST reject executable schemes and MAY restrict external resources.

### 4.1 Form node

```json
{
  "type": "form",
  "method": "POST",
  "action": "/hello",
  "fields": [{
    "name": "name",
    "type": "text",
    "label": "What should we call you?",
    "placeholder": "Your name",
    "required": true
  }],
  "submit": "Send by post"
}
```

v0.3 permits GET and POST forms and `text` fields only. Field names within a form MUST be unique. The renderer owns native controls; publishers supply no HTML or scripts.

On GET submission, the client URI-encodes field values into the action URI query and sends a bodyless GET. On POST submission, it sends field names and string values as the request JSON body. Form submissions use the selected transport exactly like other navigation.

### 4.2 Presentation Intent

A document MAY contain a `presentation` object. Presentation Intent is a constrained suggestion, never publisher CSS:

```json
{
  "accent": "#315C45",
  "background": "#FFFDF8",
  "foreground": "#17231C",
  "surface": "#F3EFE5",
  "typeface": "editorial",
  "density": "spacious",
  "corners": "soft"
}
```

Colors MUST be six-digit hexadecimal values. `typeface` is `system`, `editorial`, `sans`, or `mono`; `density` is `compact`, `comfortable`, or `spacious`; and `corners` is `square`, `soft`, or `round`.

Headings MAY suggest `variant: "display"`, images MAY suggest `variant: "hero"`, and buttons MAY suggest `variant: "prominent"`; `normal` is valid for each. These are semantic hints, not style declarations.

Clients MAY ignore or override any presentation hint. They MUST NOT treat hints as CSS, URLs, markup, executable content, or permission to fetch another resource. The publisher owns the message; the reader owns the screen.

## 5. Processing and safety

The client constructs a valid request; a transport carries it unchanged; the publisher routes method and URI, creates a correlated response, and a transport returns it unchanged. The client validates version, correlation, status, and every document node before rendering.

“Private” describes the messaging model, not a v0.3 security guarantee. Deployments obtain confidentiality, identity, integrity, and access control from their environment. Every publisher document remains untrusted.

## 6. Compatibility

v0.1 defined bodyless GET only and five non-form nodes. v0.2 added POST, JSON bodies, query semantics, and forms. v0.3 adds only optional Presentation Intent and node variants. A v0.3 document without those optional fields has the same semantic representation as v0.2. Implementations MAY continue accepting valid older request/response pairs but MUST reject mixed-version pairs and unknown fields for the declared version.

## 7. Experimental client behavior: prEmail

prEmail is optional speculative GET prefetching. It is **not part of the protocol** and a conforming client may never prefetch.

This implementation considers at most three explicit, same-origin MailWeb links after user navigation. It uses the normal request and selected transport, caches successful GET responses in memory for 60 seconds, deduplicates in-flight URIs, and never follows forms, buttons, external links, POST actions, or links discovered inside a prefetched response. Reload bypasses the cache.

Clients SHOULD expose this activity and distinguish live delivery from cached correspondence. They MUST NOT claim that a cache hit avoided the earlier transport exchange.

Speculative fetching has privacy implications: it contacts publishers for resources the user may never choose to visit. Clients SHOULD make prefetching visible and SHOULD offer policy control before prEmail progresses beyond experimentation.
