# MailWeb Protocol v0.5

## Status

This document defines the experimental MailWeb Protocol v0.5. The words **MUST**, **MUST NOT**, **SHOULD**, and **MAY** carry their usual RFC meanings.

## 1. Scope and transport independence

MailWeb describes application requests, responses, and safe semantic documents. It does not describe delivery. Identical JSON may travel through SMTP, HTTP, files, WebSockets, or another carrier. Carrier addresses, framing, retries, encryption, and connection state MUST NOT appear as protocol-specific fields.

Messages are UTF-8 JSON objects. The canonical [request](../packages/protocol/schema/mailweb-request.schema.json), [response](../packages/protocol/schema/mailweb-response.schema.json), and [document](../packages/protocol/schema/mailweb-document.schema.json) schemas are closed; unknown fields are invalid.

## 2. Requests

Every request contains `mailweb: "0.5"`, a fresh uppercase ULID `id`, `method`, an absolute `mailweb://` URI, and string-valued `headers`.

### 2.1 GET and query parameters

GET retrieves a document and MUST NOT contain `body`. Query parameters belong in the URI and use normal URI percent-encoding:

```json
{
  "mailweb": "0.5",
  "id": "01J00000000000000000000000",
  "method": "GET",
  "uri": "mailweb://demo.local/search?q=internet",
  "headers": {}
}
```

The query is part of resource identity. Publishers parse it from `uri`; no parallel query structure exists.

### 2.2 POST and request bodies

POST submits application data. In v0.4, `body` is a JSON object and is REQUIRED for POST. A JSON body SHOULD be described by `content-type: application/json`:

```json
{
  "mailweb": "0.5",
  "id": "01J00000000000000000000000",
  "method": "POST",
  "uri": "mailweb://demo.local/hello",
  "headers": {"content-type": "application/json"},
  "body": {"name": "Levi"}
}
```

Headers are MailWeb application metadata, not SMTP or HTTP headers. A client MAY advertise stationery already held in memory using `mailweb-stationery`, whose value is a JSON object mapping template IDs to exact versions. This is an application optimization hint, not carrier state. v0.4 defines no cookies, sessions, or authentication.

## 3. Responses and correlation

A response contains `mailweb: "0.5"`, `request_id`, an integer `status` from 100 through 599, and a `document`. `request_id` MUST exactly match the request `id`. Status describes the MailWeb result, not carrier delivery. A document is required even for errors.

## 4. Documents

A document contains a plain-text `title` and ordered `body`. Text is never markup. Renderers MUST escape it and MUST NOT interpret HTML, Markdown, JavaScript, templates, or event handlers.

v0.4 supports:

- `heading`: `level` 1–6 and plain `text`.
- `paragraph`: plain `text`.
- `link`: plain `label` and URI-reference `href`.
- `image`: URI-reference `src` and plain `alt`.
- `button`: navigational `label` and `href`; it executes no code and submits no data.
- `form`: semantic input and submission metadata described below.
- `nav`: an accessible label and ordered `{label, href}` items. Items navigate through MailWeb like links, and renderers SHOULD identify the current destination.
- `slot`: a template-only named insertion point. It is invalid in ordinary composed documents and response slot content.

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

v0.4 permits GET and POST forms and `text` fields only. Field names within a form MUST be unique. The renderer owns native controls; publishers supply no HTML or scripts.

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

### 4.3 Templates (“stationery”)

A response document MAY name a `template`, its exact `template_version`, and a `slots` object. The version is `sha256:` followed by the lowercase SHA-256 of the compact UTF-8 JSON serialization of the template `document`. Clients cache by the `(id, version)` pair; a different version of the same ID is not interchangeable.

On first delivery, top-level `templates` MAY enclose a matching definition containing `id`, `version`, and one complete semantic `document`. Template documents cannot reference templates. They contain ordinary safe nodes plus named `slot` nodes and use the ordinary size limits.

Composition is deterministic:

1. Validate definitions and their content-derived versions, then file them in session memory.
2. Locate the exact referenced ID and version. If absent, retain the correspondence and report `MISSING STATIONERY`; v0.4 performs no automatic fetch.
3. Reject supplied slot names not declared by the template. This prevents typos silently discarding content.
4. Walk the template body in order, replacing each slot with its supplied nodes. A missing slot inserts zero nodes. Slot content cannot contain slots.
5. Render the resulting ordinary semantic document.

Template Presentation Intent supplies defaults; response properties override those defaults; reader overrides remain authoritative. No inheritance, nested templates, loops, conditions, expressions, HTML, CSS, or executable code exist. Human-facing clients may call templates “stationery”; wire fields retain the technical word `template`.

### 4.4 Enclosures

Protocol 0.5 responses MAY declare up to 16 transport-independent `enclosures`. Each has a unique request-local `id`, sanitized basename `filename`, allowed `media_type`, byte `size`, and lowercase SHA-256 `digest`. JSON carriers represent optional bytes as base64 `content`. The logical model never mentions MIME parts or carrier URLs.

An `image` may reference an enclosure by exact `enclosure` ID and `digest`. An `attachment` node adds a plain label and optional description for an explicitly opened or downloaded file. Supported inline images are PNG, JPEG, and WebP; SVG, HTML, scripts, stylesheets, audio, video, and executables are excluded. Downloadable v0.5 enclosures are additionally limited to PDF and plain text.

Clients validate decoded byte length and digest before filing or exposing content. This implementation permits 16 enclosures, 2 MiB per enclosure, 5 MiB declared total, and 64 session-cached digests. Duplicate IDs, unsafe filenames, unsupported media, malformed base64/MIME, oversized data, and digest mismatches fail safely. Missing referenced bytes produce `MISSING ENCLOSURE`; no remote fallback occurs.

Postbox MAY advertise at most 64 digests actually available in session memory using the JSON-array `mailweb-known-resources` header. A publisher may then omit `content` while retaining the enclosure descriptor. Clients resolve only the exact digest. Stationery and response slots may reference enclosures under the same rule.

SMTP commonly maps a response to `multipart/mixed`: one `application/mailweb+json` part plus binary MIME parts associated through adapter-private Content-IDs. HTTP development transport uses JSON/base64. After decoding, both yield the same logical response. MIME identifiers are not protocol fields.

Opening a filed enclosure is a local Postbox operation and MUST NOT imply a publisher request. Renderers MUST treat bytes as untrusted, MUST NOT execute attachments, and MUST require explicit action for downloads.

## 5. Processing and safety

The client constructs a valid request; a transport carries it unchanged; the publisher routes method and URI, creates a correlated response, and a transport returns it unchanged. The client validates version, correlation, status, and every document node before rendering.

“Private” describes the messaging model, not a v0.4 security guarantee. Deployments obtain confidentiality, identity, integrity, and access control from their environment. Every publisher document remains untrusted.

## 6. Compatibility

v0.1 defined bodyless GET only and five non-form nodes. v0.2 added POST, JSON bodies, query semantics, and forms. v0.3 added optional Presentation Intent and node variants. v0.4 adds semantic navigation and reusable templates. Publishers MAY expand stationery into complete documents for older clients. Implementations MAY accept valid older pairs but MUST reject mixed-version pairs and unknown fields for the declared version.

## 7. Experimental client behavior: prEmail

prEmail is optional speculative GET prefetching. It is **not part of the protocol** and a conforming client may never prefetch.

This implementation considers at most three explicit, same-origin MailWeb links after user navigation. It uses the normal request and selected transport, caches successful GET responses in memory for 60 seconds, deduplicates in-flight URIs, and never follows forms, buttons, external links, POST actions, or links discovered inside a prefetched response. Reload bypasses the cache.

Clients SHOULD expose this activity and distinguish live delivery from cached correspondence. They MUST NOT claim that a cache hit avoided the earlier transport exchange.

Speculative fetching has privacy implications: it contacts publishers for resources the user may never choose to visit. Clients SHOULD make prefetching visible and SHOULD offer policy control before prEmail progresses beyond experimentation.
