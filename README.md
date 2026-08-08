# MailWeb

> **What if browsing the web was technically just checking your email?**

MailWeb is a deliberately absurd experiment in transporting web-style documents as private messages instead of serving them directly as HTTP responses.

It is **not an attempt to replace HTTP**, nor a claim that routing a website through email magically places it beyond regulation. Quite the opposite: MailWeb exists to explore — and gently make fun of — how blurry the boundaries between a "website", a "message", and a "protocol" become when you separate content from its transport.

A page still feels like a normal website to the person browsing it. Underneath, however, every navigation is correspondence: the client sends a message requesting a document, the publisher replies with another message, and the client renders the response.

Yes, this means the demo genuinely browses a website through email.

Yes, this is ridiculous.

**That's the point.**

MailWeb takes a mundane technical fact — that presentation and transport are separate concerns — to an unreasonable conclusion.

![Postbox rendering the MailWeb homepage after receiving it through private email correspondence](docs/images/homepage.png)

## The point

MailWeb asks a deliberately silly question:

> **If the same interactive document can arrive over HTTP, SMTP, or some other carrier without substantially changing the experience of the person using it, where exactly does a "website" begin and end?**

The project doesn't pretend to answer the legal or policy questions that follow from that. It just makes the architectural absurdity tangible.

**HTTP is not the Internet.**

SMTP is not inherently private. Changing transport does not make a service immune from law. MailWeb is not a censorship-circumvention system, anonymity network, or legal loophole.

But transport and presentation *are* separate concerns.

So MailWeb separates them, sends the web through the post, and sees what happens.

## How it works

This repository implements complete static and dynamic MailWeb round trips and deliberately keeps the machinery visible:

```text
Client
  ↓
MailWeb request
  ↓
Message transport
  ↓
Publisher
  ↓
MailWeb response
  ↓
Message transport
  ↓
Client renderer
```

In the SMTP demo, that becomes:

```text
Postbox
  ↓
SMTP
  ↓
Mailpit
  ↓
Laravel publisher
  ↓
SMTP
  ↓
Mailpit
  ↓
Postbox
```

A navigation such as:

```text
mailweb://demo.local/hello
```

produces a request conceptually like:

```json
{
  "mailweb": "0.3",
  "method": "GET",
  "uri": "mailweb://demo.local/hello"
}
```

The publisher replies with a semantic MailWeb document.

Postbox renders it.

No HTTP response was used to deliver the page.

## It's actually interactive

MailWeb isn't limited to static documents.

Protocol 0.3 supports GET requests, query parameters, POST requests, request bodies and semantic forms.

A MailWeb page can present an input field, the user can submit it, Laravel can process the submitted data, and the resulting dynamic document travels back through the same correspondence mechanism.

In the SMTP demo, that means:

```text
User fills in form
        ↓
MailWeb POST
        ↓
SMTP
        ↓
Laravel
        ↓
Dynamic response
        ↓
SMTP
        ↓
Postbox
        ↓
Rendered page
```

It is, for all practical purposes, a tiny interactive website.

It just happens to communicate by email.

## Postbox

Postbox is the reference MailWeb client.

It currently provides both graphical and terminal interfaces and is responsible for:

- MailWeb navigation
- back, forward and reload
- transport selection
- protocol validation
- safe semantic document rendering
- forms and dynamic requests
- navigation history
- speculative correspondence
- message inspection
- session-only journey tracing

The graphical Postbox deliberately exposes its machinery.

Its Message Inspector shows the exact request, response, method, request body, timing, status, transport and delivery source behind the document currently being displayed.

Its Journey Inspector pulls up the floorboards: a structured, timestamped trace of events Postbox actually observed, including archive checks, real SMTP acceptance, mailbox polling, response correlation, validation, prEmail filing and retrieval, POST submission with redacted values, and failures. It deliberately makes no claims about publisher internals it cannot observe.

Unusual technical states also receive a restrained postal interpretation—such as `RETURN TO SENDER` for a real 404 or `NO REPLY` for a real transport timeout—while the Message Inspector always preserves the actual status and protocol evidence.

The absurdity is part of the interface.

## prEmail

SMTP was not designed for interactive browsing.

This is, unsurprisingly, somewhat inconvenient when using SMTP for interactive browsing.

MailWeb therefore contains an experimental Postbox feature called **prEmail**.

After a user opens a document, Postbox may identify likely same-origin GET destinations and send correspondence to them ahead of time.

Their replies are retained briefly in the local Postbox.

If the user subsequently follows one of those links:

```text
User reads /
      ↓
Postbox notices /about
      ↓
✉ GET /about
      ↓
reply arrives
      ↓
stored in Postbox

...time passes...

User clicks About
      ↓
Already in your postbox.
      ↓
instant render
```

The underlying SMTP exchange still happened.

It simply happened *before the user asked to see the result*.

The Message Inspector deliberately exposes this distinction, including the original SMTP round-trip duration and the later effectively instant navigation.

prEmail is conservative by design. It currently considers only a small number of same-origin GET links, does not recursively crawl, does not prefetch POST requests or forms, and keeps responses only briefly in memory.

Speculative requests also have privacy implications: requesting a document may disclose interest in something the user never ultimately opens.

For that reason, prEmail's behaviour is deliberately visible rather than hidden.

And yes, the name is staying.

## Protocol and transport

MailWeb messages do not know how they are being transported.

The application contract is defined independently in `packages/protocol`.

This repository currently provides two development transports behind the same Go interface:

```text
               MailWeb
                  │
             request/response
                  │
          ┌───────┴───────┐
          │               │
        HTTP             SMTP
          │               │
       direct           Mailpit
```

HTTP exists as a fast development carrier.

SMTP exists because that's where the joke becomes real.

HTTP endpoints, SMTP addresses, MIME headers, Mailpit polling and other transport details do not appear in MailWeb request or response objects.

## Laravel

`packages/laravel-mailweb` provides the reusable Laravel implementation of MailWeb.

The demo site is intentionally an ordinary consumer of this package rather than containing a privileged first-party MailWeb implementation.

The intended developer experience is approximately:

```php
MailWeb::get('/', function () {
    return MailWeb::page('Hello MailWeb')
        ->heading('Dear Internet')
        ->paragraph('This document arrived through correspondence.')
        ->link('Say hello', '/hello');
});
```

Dynamic routes can accept MailWeb requests:

```php
MailWeb::post('/hello', function (MailWebRequest $request) {
    $name = $request->input('name');

    return MailWeb::page("Dear {$name}")
        ->heading("Dear {$name},")
        ->paragraph('Lovely to correspond.');
});
```

The Laravel package handles the protocol and transport machinery underneath that API.

The demo application therefore isn't MailWeb.

It's simply a Laravel application that happens to speak MailWeb.

## Repository layout

- `apps/postbox` — Go graphical and terminal clients, transports, protocol validator and renderers
- `apps/demo-site` — ordinary Laravel application consuming the reusable package
- `packages/protocol` — protocol specification, JSON Schema and examples
- `packages/laravel-mailweb` — reusable Composer package implementing MailWeb 0.3 for Laravel
- `extension` — reserved for future browser integration

## Browse locally

Docker and Docker Compose are the only host requirements.

### Graphical Postbox

Start the graphical browser using SMTP as its carrier:

```bash
./postbox-ui
```

Open:

```text
http://127.0.0.1:9847
```

Then enter:

```text
mailweb://demo.local/
```

and press **Send**.

Every live graphical navigation travels out and back through Mailpit.

Visit:

```text
mailweb://demo.local/hello
```

to submit a semantic form as a real MailWeb POST and receive Laravel's personalised response.

The graphical client includes:

- back
- forward
- reload
- MailWeb address bar
- safe structured document rendering
- semantic forms
- message inspector
- request/response timing
- transport visibility
- prEmail status and cache visibility
- live send/wait/receive correspondence animations
- a session-only correspondence archive
- visibly distinct retrieval of already-received letters
- constrained publisher Presentation Intent with reader overrides
- Correspondence View translating the real exchange into familiar letters
- Journey Inspector with real event history and expandable protocol evidence
- postal state treatments for returned, delayed, declined, damaged, and unavailable correspondence
- full reduced-motion support

Set:

```bash
MAILWEB_UI_TRANSPORT=http
```

when launching to use the direct development carrier instead.

### Terminal Postbox

MailWeb does not require a graphical renderer.

Launch the terminal client with:

```bash
./postbox mailweb://demo.local/
```

Enter a displayed number to navigate between documents, or `q` to quit.

To make every navigation travel out and back as actual email through SMTP:

```bash
./postbox --transport smtp mailweb://demo.local/
```

Postbox prints:

```text
Waiting for the publisher to reply...
```

while awaiting its correspondence.

The default reply timeout is 15 seconds and can be changed with:

```bash
./postbox --transport smtp --timeout 30s mailweb://demo.local/
```

### Watching the post

Mailpit exposes its development interface at:

```text
http://localhost:8025
```

Open it while browsing through Postbox to watch the underlying request and response messages arrive.

This is highly recommended.

It makes the whole thing considerably more ridiculous.

### Direct publisher debugging

The publisher also exposes a local HTTP development endpoint:

```bash
curl -s http://localhost:8081/mailweb/messages \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d @packages/protocol/examples/request.json
```

This bypasses SMTP and uses the direct development transport.

The publisher's HTTP port exists only as a local debugging convenience.

## Local development

Compose bind-mounts `apps/postbox` at `/app` in the Go container and `apps/demo-site` at `/app` in the Laravel container.

Each `./postbox` invocation compiles the current Go source.

Laravel reads PHP files on each request.

Laravel's generated `vendor` directory is kept in the Docker-managed `demo-site-vendor` volume rather than being written into the repository.

## A note on "private"

MailWeb uses **private message** to describe its correspondence model: a request is addressed to a publisher and the resulting response is addressed back to a specific client mailbox.

SMTP does **not** inherently provide end-to-end confidentiality or anonymity.

Mail infrastructure may be able to observe message contents, metadata, sender and recipient information unless additional protections are used.

MailWeb currently makes no claim otherwise.

Future experiments may explore end-to-end encrypted correspondence, but encryption is deliberately outside the current protocol experiment.

## Security model

MailWeb publishers do not send arbitrary HTML or JavaScript for Postbox to execute.

Documents are semantic structures containing supported node types such as headings, paragraphs, links, images and forms.

The client owns rendering.

This is intentional.

It keeps the experiment focused on documents and transport rather than recreating the entire browser security model inside a weekend project that sends websites through email.

## Scope

MailWeb deliberately does **not** currently provide:

- authentication
- end-to-end encryption
- anonymity
- persistent caching
- service discovery
- arbitrary publisher JavaScript
- arbitrary remote HTML
- a production mail service
- a censorship-circumvention system
- immunity from regulation
- a replacement for HTTP

There is intentionally no database, queue, event bus, persistent cache or orchestration beyond Compose.

Postbox's deliberately primitive correspondence archive exists only within one process and expires GET replies after 60 seconds. prEmail files its speculative replies into that same archive.

## Status

MailWeb is experimental.

The protocol will change.

The implementation will contain questionable decisions.

SMTP will occasionally remind everyone why it was not selected as the transport for the World Wide Web.

That's fine.

The purpose of MailWeb is not to demonstrate that email is better than HTTP.

It's to demonstrate that **the Internet is not HTTP**, that presentation and transport are different layers, and that apparently obvious boundaries become much less obvious when you start moving those layers around.

Or, put another way:

> **This is ridiculous. That's the point.**
