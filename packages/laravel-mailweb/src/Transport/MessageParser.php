<?php

namespace MailWeb\Laravel\Transport;

use MailWeb\Laravel\Protocol\ProtocolException;

final class MessageParser
{
    public function payload(string $raw): string
    {
        $parts = preg_split("/\r?\n\r?\n/", $raw, 2);
        if (count($parts) !== 2) { throw new ProtocolException('Malformed email message.'); }
        $headers = preg_replace("/\r?\n[ \t]+/", ' ', $parts[0]);
        if (! preg_match('/^Content-Type:\s*application\/mailweb\+json(?:\s*;|\s*$)/mi', (string) $headers)) {
            throw new ProtocolException('Request is not application/mailweb+json.');
        }
        $body = $parts[1];
        if (preg_match('/^Content-Transfer-Encoding:\s*base64\s*$/mi', (string) $headers)) {
            $decoded = base64_decode($body, true);
            if ($decoded === false) { throw new ProtocolException('Invalid base64 message body.'); }
            return $decoded;
        }
        if (preg_match('/^Content-Transfer-Encoding:\s*quoted-printable\s*$/mi', (string) $headers)) {
            return quoted_printable_decode($body);
        }
        return $body;
    }
}
