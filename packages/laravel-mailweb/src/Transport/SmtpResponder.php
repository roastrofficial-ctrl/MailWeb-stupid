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
		$parts = [];
		foreach ($response['enclosures'] ?? [] as &$enclosure) { if (isset($enclosure['content'])) { $parts[] = ['id' => $enclosure['id'], 'filename' => $enclosure['filename'], 'media_type' => $enclosure['media_type'], 'digest' => $enclosure['digest'], 'content' => $enclosure['content']]; unset($enclosure['content']); } }
		unset($enclosure);
        $json = json_encode($response, JSON_THROW_ON_ERROR | JSON_UNESCAPED_SLASHES);
		$headers = "From: {$this->sender}\r\nTo: {$recipient}\r\n"
            ."Subject: MailWeb response {$response['request_id']}\r\nMIME-Version: 1.0\r\n"
			;
		if ($parts === []) { $raw = $headers."Content-Type: application/mailweb+json; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n{$json}"; }
		else {
			$boundary = 'mailweb-'.bin2hex(random_bytes(12)); $raw = $headers."Content-Type: multipart/mixed; boundary=\"{$boundary}\"\r\n\r\n";
			$raw .= "--{$boundary}\r\nContent-Type: application/mailweb+json; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n{$json}\r\n";
			foreach ($parts as $part) { $filename = addcslashes($part['filename'], "\"\\"); $raw .= "--{$boundary}\r\nContent-Type: {$part['media_type']}\r\nContent-ID: <{$part['id']}>\r\nX-MailWeb-Digest: {$part['digest']}\r\nContent-Disposition: attachment; filename=\"{$filename}\"\r\nContent-Transfer-Encoding: base64\r\n\r\n".chunk_split($part['content'], 76, "\r\n"); }
			$raw .= "--{$boundary}--\r\n";
		}
        $envelope = new Envelope(new Address($this->sender), [new Address($recipient)]);
        (new Mailer(new EsmtpTransport($this->host, $this->port, false)))->send(new RawMessage($raw), $envelope);
    }
}
