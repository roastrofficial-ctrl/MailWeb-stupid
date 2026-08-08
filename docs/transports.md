# Development transports

MailWeb Protocol v0.1 defines request and response JSON only. Transports are replaceable carriers for those messages.

## Direct HTTP

The `HTTPTransport` sends the request JSON directly to the Laravel development endpoint and reads response JSON synchronously. It is the fastest feedback loop:

```bash
./postbox --transport http mailweb://demo.local/
```

## SMTP through Mailpit

The `SMTPTransport` creates a unique client mailbox for each request, sends an email to `browse@demo.local`, and polls that mailbox through Mailpit's local API. The Laravel `mailweb:consume` worker reads publisher mail, routes the MailWeb URI, and sends a response email back through SMTP.

Both email bodies have media type `application/mailweb+json`. Email subjects, sender and recipient addresses, transfer encoding, SMTP delivery, mailbox polling, and timeout behavior belong to this transport and are not fields in the MailWeb protocol.

```text
postbox
  -> SMTP request email
  -> Mailpit
  -> Laravel mailbox worker
  -> SMTP response email
  -> Mailpit
  -> postbox mailbox poll
```

Run it with:

```bash
./postbox --transport smtp mailweb://demo.local/
```

Mailpit's development UI is available at <http://localhost:8025>.
