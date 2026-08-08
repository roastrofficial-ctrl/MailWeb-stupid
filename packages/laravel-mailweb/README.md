# Laravel MailWeb

Publish safe, semantic [MailWeb Protocol 0.2](../../docs/protocol.md) documents from an ordinary Laravel application. The package handles protocol envelopes, validation, request correlation, routing, serialization, inbound correspondence, and SMTP replies. Your application writes pages.

MailWeb is experimental art/software. It sends website-like interactions as private messages and should not be treated as production security infrastructure.

## Quick start

```bash
composer require mailweb/laravel-mailweb
php artisan vendor:publish --tag=mailweb-config
```

Laravel auto-discovers the service provider and facade. Create `routes/mailweb.php`:

```php
use MailWeb\Laravel\Facades\MailWeb;

MailWeb::get('/', fn () => MailWeb::page('Hello MailWeb')
    ->heading('Dear Internet')
    ->paragraph('This document was delivered through correspondence.')
    ->link('Say hello', '/hello'));
```

Configure an address and start listening:

```dotenv
MAILWEB_PUBLISHER_MAILBOX=browse@example.test
MAILWEB_SMTP_HOST=localhost
MAILWEB_SMTP_PORT=1025
MAILWEB_INBOUND_DRIVER=mailpit
MAILWEB_MAILPIT_URL=http://localhost:8025
```

```bash
php artisan mailweb:listen
```

Your MailWeb client can now request the publisher. Carrier-to-domain discovery is intentionally outside Protocol 0.2, so configure the client destination separately.

## Configuration

The published `config/mailweb.php` contains only the current integration boundaries:

- `protocol` — current package protocol version (`0.2`).
- `routes` — route declaration file, normally `routes/mailweb.php`.
- `http_endpoint` — optional direct/local carrier endpoint, default `/mailweb/messages`.
- `publisher_address` — sender and inbound publisher mailbox.
- `inbound` — inbox driver, API endpoint, and polling interval.
- `smtp` — generic SMTP host and port used for replies.

Mailpit is the included development inbox driver, not part of MailWeb. You can bind your own implementation of `MailWeb\Laravel\Transport\Inbox` for a different mailbox source. Outbound replies use ordinary SMTP settings.

## Routes and requests

GET and POST routes accept closures, invokable classes, and Laravel-style controller actions:

```php
use MailWeb\Laravel\Http\MailWebRequest;

MailWeb::get('/search', function (MailWebRequest $request) {
    return MailWeb::page('Search')
        ->paragraph('You searched for '.$request->query('q'));
});

MailWeb::get('/people/{name}', [PeopleController::class, 'show']);

MailWeb::post('/hello', function (MailWebRequest $request) {
    $name = $request->input('name');

    return MailWeb::page("Dear {$name}")
        ->heading("Dear {$name},")
        ->paragraph('Lovely to correspond.')
        ->link('Write again', '/hello');
});
```

`MailWebRequest` provides:

```php
$request->method();
$request->uri();
$request->path();
$request->input('name');
$request->query('q');
$request->all();
$request->header('content-type');
```

Malformed envelopes and unsupported versions are rejected before route handlers run. Query parameters stay in the MailWeb URI; POST application data is a JSON object.

## Forms and dynamic content

Publish semantic fields rather than HTML:

```php
MailWeb::get('/hello', fn () => MailWeb::page('Hello')
    ->heading('What is your name?')
    ->form(
        method: 'POST',
        action: '/hello',
        fields: [
            MailWeb::text(
                name: 'name',
                label: 'What should we call you?',
                placeholder: 'Your name',
                required: true,
            ),
        ],
        submit: 'Send by post',
    ));
```

The client owns the input controls. A GET form URI-encodes fields into its action query. A POST form sends them as the MailWeb JSON body through whichever transport the client selected.

## Document builder

`MailWeb::page($title, $status = 200)` supports every Protocol 0.2 node:

```php
MailWeb::page('A letter', status: 200)
    ->heading('Heading', level: 1)
    ->paragraph('Plain text only.')
    ->link('Continue', '/next')
    ->button('Open', '/next')
    ->image('https://example.test/stamp.png', 'A postage stamp')
    ->form('POST', '/reply', [MailWeb::text('message', 'Message')], 'Post it');
```

`add(array $node)` is available for low-level protocol work. Prefer the typed helpers in application code.

## Local HTTP carrier

The package registers `POST /mailweb/messages` by default. This accepts exactly the same MailWeb JSON envelope and is useful for direct local clients and tests. Set `http_endpoint` to `null` to disable it. HTTP is a carrier here; its concepts do not enter the MailWeb envelope.

## Local Mailpit development

Run Mailpit with SMTP on port 1025 and its development API on port 8025, configure the variables above, then run `php artisan mailweb:listen`. The listener polls the configured publisher mailbox, accepts `application/mailweb+json`, validates and dispatches the request, and sends a correlated `application/mailweb+json` reply through SMTP.

Use `php artisan mailweb:listen --once` to process currently available messages and exit. No queue or database is required.

## Security and limitations

- Publisher content is semantic data, never arbitrary HTML or JavaScript.
- Protocol 0.2 does not provide encryption, authentication, access control, replay protection, sessions, or production mailbox discovery.
- The Mailpit inbox driver is for local experimentation. Bind a suitable `Inbox` implementation before using another receiving system.
- Validate and constrain all user input as you would in any Laravel application.
- A transport is responsible for whatever confidentiality and identity guarantees it claims to provide.

Please preserve the important implementation detail: somewhere, against its better judgment, a computer is waiting for a website to arrive in the post.
