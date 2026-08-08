# MailWeb

## Disclaimer

There's a fine line between absurd and profound. Skirting that line is very human, and so is this experiment. The contents of the repo are not all human, though. I used AI to help develop, refine and test this monster. 

I'm a senior Laravel, architecty guy; so the planning, features and **most** engineering was me. The grunt work was my hard working agent (including the awful job of taking my terse, borderline illegible explanation scribbled hastily into several Obsidian documents and expanding it into this readme. I leave it in tact, em dashes and all because like most engineers, I don't do words so good.)

This is a WiP. I'll keep fruitlessly working on this because I find the paradigm shift intersting, engaging and funny. 

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
  "mailweb": "0.5",
  "method": "GET",
  "uri": "mailweb://demo.local/hello"
}
```

The publisher replies with a semantic MailWeb document.

Postbox renders it.

No HTTP response was used to deliver the page.

## It's actually interactive

MailWeb isn't limited to static documents.

Protocol 0.5 supports GET, POST, semantic forms, navigation, reusable stationery, and content-addressed file enclosures.

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

### Stationery

MailWeb publishers may enclose safe reusable page structure once, identified by a content-derived SHA-256 version. Postbox files that stationery in session memory. Later correspondence carries only page-specific slot content plus the exact template reference; Postbox composes the complete semantic document locally before either renderer sees it.

Dear Internet demonstrates this with one shared identity, navigation, content slot, footer, and Presentation Intent. Open **Your Postbox** to see correspondence and stationery kept as separate kinds of mail. The Journey Inspector records whether stationery was enclosed and filed or found and reused.

Templates contain no HTML, Blade, CSS, JavaScript, expressions, loops, inheritance, or executable components. Unknown supplied slots fail validation; missing slots render nothing; an unavailable exact version produces a safe `MISSING STATIONERY` state.

### Enclosures

Protocol 0.5 allows images and explicitly downloadable files to arrive inside the same reply. SMTP uses real `multipart/mixed` attachments; the HTTP development carrier uses the equivalent JSON/base64 representation. Postbox reconstructs one logical response, verifies every byte count and SHA-256 digest, and files valid resources in a separate in-memory **ENCLOSURES** archive.

The Dear Internet homepage’s hero PNG and unnecessary plain-text manifesto are genuine attachments. The graphical renderer obtains them only from Postbox’s loopback enclosure endpoint. About reuses the same image by digest without retransmitting its 1.8 MB of bytes, and prEmail can file both correspondence and resources before a click.

No SVG, HTML, JavaScript, CSS, executable files, audio, video, remote asset fallback, or automatic opening is supported. Opening an attachment is a local reader action and creates no new publisher request.

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
- `packages/laravel-mailweb` — reusable Composer package implementing MailWeb 0.5 for Laravel
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
- semantic stationery filed and reused by exact content identity
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

... oh, you're still here... enjoy these rambling (pseudo)philosophical thoughts on this abomination.

## Questions MailWeb accidentally asks

MailWeb began with a deliberately ridiculous premise:

> What if browsing the web was technically just checking your email?

Actually implementing that premise has produced some less ridiculous questions.

MailWeb is not presented as the answer to any of them. Most of these ideas have analogues in existing distributed systems, privacy systems, capability models, message-oriented architectures and networking research.

But making **message passing the browsing model** puts several normally invisible assumptions about the Web in an unusual light.

So, having built the stupid thing, here are some of the questions it has started asking.

### What if a website had no public web server?

A MailWeb publisher does not necessarily need to expose its application directly to the Internet.

A deployment could look like:

```text
Internet
   |
   v
mail infrastructure
   |
   v
MailWeb validator
   |
   v
message boundary
   |
   v
private application
```

The application could have no publicly accessible HTTP interface at all.

This does not make it unhackable.

It moves the attack surface.

Mail servers, MIME parsers, protocol parsers, validators, queues and the application itself all remain software and therefore remain capable of having vulnerabilities.

But it raises an interesting architectural question:

> What happens to the security model of a web application when interacting with it no longer requires exposing the application itself to the public network?

Message queues, gateways and brokers already create boundaries like this.

MailWeb's only unusual contribution is making that boundary part of the browsing model.

### What if opening a website didn't mean executing somebody else's program?

The modern Web routinely delivers executable code to the reader.

MailWeb currently does not.

A publisher sends semantic intent:

```text
heading
paragraph
navigation
image
form
button
template
presentation hint
```

Postbox decides how those things behave and how they are rendered.

That produces a very different trust boundary.

A correspondent can ask Postbox to display a button.

They cannot define arbitrary JavaScript for that button to execute.

A correspondent can suggest presentation.

They do not own the reader's screen.

This raises a broader question:

> How much of the modern browser security problem exists because documents evolved into remotely supplied applications?

Or, in MailWeb terminology:

> The correspondent doesn't get to execute code in your house merely because you opened their letter.

### What if browsing didn't require the reader and publisher to know each other's network address?

MailWeb does not eliminate IP networking.

The infrastructure underneath still needs networks, addresses and routing.

But the **reader and publisher do not necessarily need a direct network relationship with each other**.

A reader could communicate through a Post Office or relay:

```text
Reader
   |
   v
Post Office
   |
   | correspondence
   v
Publisher
```

and the reply can return through the same messaging infrastructure.

The publisher needs a return address.

It does not inherently need the reader's IP address.

Likewise, the reader needs a MailWeb address.

It does not inherently need to establish a connection to the publisher's application server.

That suggests a more precise question than "IP-less browsing":

> What would browsing look like if network location stopped being part of the relationship between reader and publisher?

The Internet underneath still has IP addresses.

The correspondence above it might not need to care.

### What if identity were scoped to a conversation?

An ordinary email address is usually a terrible anonymity mechanism.

MailWeb does not change that.

But a return address does not necessarily need to represent a permanent identity.

Postbox could theoretically use:

```text
one permanent address

or

one address per correspondent

or

one address per browsing session

or

one address capable of receiving exactly one reply
```

A publisher might therefore know:

```text
reply-to: 7f82a1@postbox.example
```

without that address necessarily being useful for correlating the same reader across unrelated correspondents.

This is not anonymity by itself.

Mail providers, relays, timing information and other metadata can still reveal or correlate activity.

But it raises another question:

> What if identity on the Internet were something the client deliberately scoped rather than something every service tried to reconstruct?

### What if the browser owned the copy it received?

Web browsing usually treats a page as something that exists somewhere else.

You request it, render it, perhaps cache it, and request it again later.

Correspondence suggests a subtly different model:

> Somebody sent you something.

Postbox already contains received correspondence and reusable stationery.

That naturally raises the possibility that previously received documents belong to the reader's local archive.

A publisher can send a newer version.

It cannot retroactively alter the copy you previously received.

Conceptually:

```text
ABOUT

8 August 2026
current correspondence

2 August 2026
previous correspondence

27 July 2026
previous correspondence
```

This starts to make browser history look less like a list of places visited and more like an archive of information actually received.

Which raises a surprisingly philosophical question:

> When you read something on the Internet, should you retain the thing you read, or merely the address where it used to exist?

### What if history could also deliberately disappear?

Correspondence naturally creates an archive.

But because Postbox owns that archive, retention could also become an explicit reader decision.

For example:

```text
RETENTION

Keep correspondence
Keep for this session
Destroy after reading
Destroy after 24 hours
```

This would not magically erase copies, metadata or logs held elsewhere in the delivery chain.

It is not guaranteed anonymity or forensic invisibility.

But it suggests a cleaner local model for private browsing:

> Incognito could mean choosing not to retain the correspondence, rather than attempting to clean up dozens of different kinds of browser state afterwards.

### What if "loading a page" didn't require waiting?

HTTP browsing strongly encourages a synchronous mental model:

```text
request
   |
   wait
   |
response
```

Correspondence does not.

It naturally permits:

```text
request

...go and do something else...

response
```

MailWeb currently tries to make correspondence fast enough to feel like conventional browsing.

But perhaps that is the wrong thing to optimise for every interaction.

Some requests are naturally asynchronous:

```text
"Tell me when this train platform is announced."

"Send me the finished report."

"Let me know when these tickets become available."

"Tell me when this job finishes."

"Send me the result when the render completes."

"Reply when somebody approves this request."

"Tell me when this parcel changes state."

"Send me tomorrow's forecast when it becomes relevant."
```

Today these experiences are implemented using mixtures of polling, background jobs, WebSockets, push notification infrastructure, email, mobile notifications and application-specific state.

A correspondence-native protocol makes another model possible:

```text
request
   |
   v
202 — correspondence accepted

Postbox closes the interaction.

...four hours later...

new correspondence arrives
```

At that point the distinction between a website response and a notification becomes strangely blurry.

Which raises the question:

> Why must a response arrive while the person who requested it is still waiting?

### What if subscriptions were just permission to keep writing?

Push notifications are usually a separate subsystem bolted onto an application.

Correspondence already has a destination.

A future MailWeb interaction could theoretically include an explicit permission:

```text
You may write to me again about this request:

[x] when its status changes

Expires:
tomorrow
```

The publisher now has a narrowly scoped capability to send further correspondence.

The reader can revoke it.

This turns "push notifications" into something conceptually simpler:

> I gave this correspondent permission to write back.

The difficult questions — spam, authentication, revocation, abuse and privacy — remain very real.

But the interaction model becomes unusually understandable.

### What if a browser cache were literally a postbox?

prEmail already performs speculative correspondence.

It requests likely destinations ahead of time and files their replies locally.

When the reader later selects one, Postbox does not need to contact the publisher:

```text
You click About

Postbox checks the archive

Correspondence already received.

Open.
```

MailWeb 0.4 extends the same metaphor to templates.

A correspondent sends its stationery once.

Later letters contain only the changing content.

The result is technically recognisable as caching and template reuse.

But the correspondence model makes both concepts surprisingly tangible:

> We already received that letter.

and:

> We already have their stationery.

This raises another question:

> How many complicated browser concepts become easier to understand when they are represented as things the reader actually possesses?

### What if email were an application message bus?

This is probably a terrible idea.

Which means MailWeb is contractually obligated to ask it.

SMTP is already a federated store-and-forward messaging system.

It provides concepts including:

- globally routable addresses
- asynchronous delivery
- intermediaries
- retries
- buffering
- federation between independently operated systems
- infrastructure that has existed for decades

Application developers routinely introduce RabbitMQ, Kafka, SQS and other messaging systems when they need asynchronous communication.

Those systems provide capabilities and guarantees that email absolutely does not magically reproduce: ordering, consumer groups, high-throughput logs, transactional semantics and many other things matter enormously.

SMTP is not Kafka.

Please do not replace Kafka with Postfix because of this README.

But MailWeb still makes the comparison difficult to ignore:

```text
Application A
     |
     | message
     v
address@example.net
     |
     | store and forward
     v
Application B
```

So there is an entertaining technical question hiding underneath the terrible engineering proposal:

> How much application messaging infrastructure already exists inside the world's email infrastructure?

And, perhaps more dangerously:

> What is the smallest useful message-oriented application you could build using nothing but ordinary email semantics?

No production systems were harmed in the asking of this question.

Yet.

### Does a MailWeb page actually exist?

This may be the strangest question the experiment has produced.

The conventional Web encourages us to think:

```text
URL -> resource
```

MailWeb increasingly looks more like:

```text
address + request
       |
       v
correspondence
       |
       v
reply
```

A MailWeb URI may therefore be less like the location of a document and more like a question you can ask a correspondent.

The answer might depend on:

- who asked
- when they asked
- previous correspondence
- what stationery they already possess
- the contents of their request
- whether Postbox asked speculatively
- whether the answer had already arrived

The resulting document can then continue to exist in the reader's Postbox even if the publisher subsequently changes or disappears.

So:

> Is MailWeb retrieving resources?

or:

> Is MailWeb conducting conversations that happen to render as documents?

We genuinely don't know which description is more useful yet.

### What if websites were correspondence rather than remote places?

That may be the broader question underneath all of this.

MailWeb began by separating transport from presentation.

Implementing it has started separating several other things that conventional browsing tends to bundle together:

```text
network location      != identity

document              != executable application

publisher intent      != client presentation

request               != synchronous waiting

navigation            != direct connection

history               != list of URLs

cache                 != invisible implementation detail

response              != something that must arrive immediately
```

None of these observations demonstrate that MailWeb is better than the Web.

In many cases the conventional Web made its trade-offs for extremely good reasons.

MailWeb has its own unpleasant questions around latency, spam, abuse, storage, freshness, large media, authentication, delivery guarantees, traffic analysis, discovery, revocation and realtime interaction.

But that is becoming the interesting part of the experiment.

The original joke was:

> **What if websites arrived by email?**

The more useful question may turn out to be:

> **Which properties of the Web are fundamental to browsing, and which are simply consequences of the architecture we happened to build it with?**

MailWeb does not have an answer.

For the moment, it is enjoying making the question unnecessarily tangible.
