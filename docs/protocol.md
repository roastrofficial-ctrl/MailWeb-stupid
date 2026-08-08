# MailWeb Protocol v0.1

## Status of this document

This document defines the experimental MailWeb Protocol v0.1. It is intentionally small and is expected to change as the first implementations are exercised.

The key words **MUST**, **MUST NOT**, **REQUIRED**, **SHOULD**, **SHOULD NOT**, and **MAY** describe requirements in the usual RFC sense.

## 1. Purpose

MailWeb transports requests for web-style documents and the corresponding document responses as private messages. The protocol defines the content of those messages only.

A MailWeb message MUST NOT depend on its carrier. The same request or response can be carried through SMTP, HTTP, a file, a WebSocket, or another transport without changing any protocol field. Addressing, delivery, retries, framing, privacy, and encryption are transport concerns and are outside v0.1.

## 2. Representation

Messages are JSON objects encoded as UTF-8. Implementations MUST reject messages that do not satisfy the applicable JSON Schema.

The canonical schemas are:

- [`MailWebRequest`](../packages/protocol/schema/mailweb-request.schema.json)
- [`MailWebResponse`](../packages/protocol/schema/mailweb-response.schema.json)
- [`MailWebDocument`](../packages/protocol/schema/mailweb-document.schema.json)

All objects are closed: fields not defined by the schemas are invalid. This keeps v0.1 deterministic while it is experimental.

## 3. MailWebRequest

A request asks for one document.

```json
{
  "mailweb": "0.1",
  "id": "01J00000000000000000000000",
  "method": "GET",
  "uri": "mailweb://demo.local/",
  "headers": {}
}
```

`mailweb` identifies the protocol version and MUST be `0.1`.

`id` is the request correlation identifier. It MUST be a canonical 26-character uppercase ULID. A sender MUST assign a new ID to each logical request. How transports deduplicate or retry a message is outside this specification.

`method` MUST be `GET`. v0.1 has no request body and defines no mutating operation.

`uri` is the resource identifier and MUST be an absolute URI using the `mailweb` scheme. It identifies the publisher by authority and the document by path, query, and fragment. A transport MAY use the authority to choose a destination, but it MUST deliver the original URI unchanged.

`headers` contains optional MailWeb metadata as string pairs. Header names are case-insensitive. Headers describe the request and MUST NOT contain carrier-specific routing or connection state. v0.1 defines no standard headers.

## 4. MailWebResponse

A response resolves exactly one request.

```json
{
  "mailweb": "0.1",
  "request_id": "01J00000000000000000000000",
  "status": 200,
  "document": {
    "title": "Hello from MailWeb",
    "body": []
  }
}
```

`request_id` MUST exactly equal the `id` of the request being answered. This is the only request/response correlation mechanism defined by v0.1.

`status` is an integer from 100 through 599. Its broad meaning follows familiar HTTP status classes: 2xx success, 3xx redirection, 4xx request failure, and 5xx publisher failure. MailWeb status is document metadata; it is not the status of the underlying transport. A transport-level success does not imply a MailWeb status of 200, and a MailWeb status of 404 does not imply a carrier failure.

`document` is REQUIRED for every response, including error responses, so a client always has something safe to render.

## 5. MailWebDocument

A document has a plain-text `title` and an ordered `body` of nodes. Clients SHOULD use the title as the view or window title. Clients MUST render body nodes in array order.

Text values are plain Unicode text. They are never markup. A renderer MUST escape them for its output environment and MUST NOT interpret HTML, Markdown, JavaScript, template syntax, or event handlers contained in text.

v0.1 supports five node types:

### 5.1 heading

A heading contains `text` and an integer `level` from 1 through 6. Level 1 is the most prominent.

### 5.2 paragraph

A paragraph contains plain `text`.

### 5.3 link

A link contains a plain-text `label` and an `href` URI reference. Activating it requests or opens the referenced resource according to client policy.

### 5.4 image

An image contains a `src` URI reference and plain-text `alt` text. Clients that cannot or will not load the image MUST present the alternative text.

### 5.5 button

A button contains a plain-text `label` and an `href` URI reference. In v0.1 a button is navigational: activating it has the same protocol effect as activating a link. It does not run code and does not submit data.

Relative URI references in `href` and `src` are resolved against the request URI. Clients MUST NOT execute `javascript:` references or other executable URI schemes. Clients MAY restrict external schemes and resources according to local policy.

## 6. Processing model

1. A client creates a valid `MailWebRequest` with a fresh ID.
2. A transport delivers the request to a publisher without altering its fields.
3. The publisher resolves the URI and creates one valid `MailWebResponse` containing the request ID.
4. A transport returns the response without altering its fields.
5. The client verifies the version and correlation ID, then renders the structured document according to the response status.

The protocol makes no claim about whether these steps are synchronous, asynchronous, online, queued, or file-based.

## 7. Security and privacy

The word “private” describes the intended message model, not a security guarantee in v0.1. Confidentiality, authentication, integrity, sender identity, access control, and replay protection are not protocol features yet. A deployment MUST obtain any required guarantees from its chosen transport or environment.

Structured nodes deliberately exclude arbitrary HTML and JavaScript. Renderers MUST still escape all text, validate URI schemes, limit resource use, and treat every received document as untrusted input.

## 8. Extensibility

Unknown fields and unknown node types are invalid in v0.1. A future protocol version can add capabilities alongside an explicit version change. Implementations MUST NOT silently reinterpret a message from an unsupported version.
