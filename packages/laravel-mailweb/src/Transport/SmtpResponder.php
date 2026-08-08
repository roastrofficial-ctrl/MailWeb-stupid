<?php

namespace MailWeb\Laravel\Transport;

use JsonException;
use Symfony\Component\Mailer\Envelope;
use Symfony\Component\Mailer\Mailer;
use Symfony\Component\Mailer\Transport\Smtp\EsmtpTransport;
use Symfony\Component\Mime\Address;
use Symfony\Component\Mime\RawMessage;

final readonly class SmtpResponder
{
    public function __construct(private string $host, private int $port, private string $sender) {}

    /** @param array<string, mixed> $response
     *  @throws JsonException
     */
    public function send(string $recipient, array $response): void
    {
        $json = json_encode($response, JSON_THROW_ON_ERROR | JSON_UNESCAPED_SLASHES);
        $raw = "From: {$this->sender}\r\nTo: {$recipient}\r\n"
            ."Subject: MailWeb response {$response['request_id']}\r\nMIME-Version: 1.0\r\n"
            ."Content-Type: application/mailweb+json; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n{$json}";
        $envelope = new Envelope(new Address($this->sender), [new Address($recipient)]);
        (new Mailer(new EsmtpTransport($this->host, $this->port, false)))->send(new RawMessage($raw), $envelope);
    }
}
