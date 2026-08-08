<?php

namespace App\MailWeb;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Http;
use JsonException;
use RuntimeException;
use Symfony\Component\Mailer\Mailer;
use Symfony\Component\Mailer\Envelope;
use Symfony\Component\Mailer\Transport\Smtp\EsmtpTransport;
use Symfony\Component\Mime\Address;
use Symfony\Component\Mime\RawMessage;
use Throwable;

final class InboxConsumer
{
    public function __construct(private readonly Publisher $publisher) {}

    public function consume(Command $command): never
    {
        $command->info('Watching browse@demo.local for MailWeb requests...');
        while (true) {
            try {
                $this->consumeAvailable($command);
            } catch (Throwable $error) {
                $command->error($error->getMessage());
            }
            usleep(500_000);
        }
    }

    private function consumeAvailable(Command $command): void
    {
        $api = rtrim((string) env('MAILWEB_MAILPIT_URL', 'http://mailpit:8025'), '/');
        $mailbox = (string) env('MAILWEB_PUBLISHER_MAILBOX', 'browse@demo.local');
        $search = Http::timeout(3)->get("{$api}/api/v1/search", [
            'query' => "to:{$mailbox} is:unread",
            'limit' => 20,
        ])->throw()->json('messages', []);

        foreach (array_reverse($search) as $summary) {
            $id = $summary['ID'] ?? null;
            if (! is_string($id)) {
                continue;
            }
            $this->markRead($api, $id);
            try {
                $message = Http::timeout(3)->get("{$api}/api/v1/message/{$id}")->throw()->json();
                $sender = $message['From']['Address'] ?? null;
                if (! is_string($sender) || $sender === '') {
                    throw new RuntimeException("Mailpit message {$id} has no sender mailbox");
                }
                $raw = Http::timeout(3)->get("{$api}/api/v1/message/{$id}/raw")->throw()->body();
                $request = $this->parseRequest($raw);
				$response = $this->publisher->respond($request);
                $this->sendResponse($sender, $response);
				$command->line("Replied to {$sender} for {$request->method} {$request->uri}");
            } catch (Throwable $error) {
                $command->error("Skipped Mailpit message {$id}: {$error->getMessage()}");
            }
        }
    }

	private function parseRequest(string $raw): MailWebRequest
    {
        $parts = preg_split("/\r?\n\r?\n/", $raw, 2);
        if (count($parts) !== 2) {
            throw new RuntimeException('Malformed email message');
        }
        $headers = preg_replace("/\r?\n[ \t]+/", ' ', $parts[0]);
        if (! preg_match('/^Content-Type:\s*application\/mailweb\+json(?:\s*;|\s*$)/mi', $headers)) {
            throw new RuntimeException('Request is not application/mailweb+json');
        }
        $body = $parts[1];
        if (preg_match('/^Content-Transfer-Encoding:\s*base64\s*$/mi', $headers)) {
            $body = base64_decode($body, true);
            if ($body === false) {
                throw new RuntimeException('Invalid base64 message body');
            }
        } elseif (preg_match('/^Content-Transfer-Encoding:\s*quoted-printable\s*$/mi', $headers)) {
            $body = quoted_printable_decode($body);
        }
        try {
            $request = json_decode($body, true, flags: JSON_THROW_ON_ERROR);
        } catch (JsonException $error) {
            throw new RuntimeException('Invalid MailWebRequest JSON', previous: $error);
        }
		return MailWebRequest::fromArray($request);
    }

    private function sendResponse(string $recipient, array $response): void
    {
        $host = (string) env('MAILWEB_SMTP_HOST', 'mailpit');
        $port = (int) env('MAILWEB_SMTP_PORT', 1025);
        $json = json_encode($response, JSON_THROW_ON_ERROR | JSON_UNESCAPED_SLASHES);
        $sender = (string) env('MAILWEB_PUBLISHER_MAILBOX', 'browse@demo.local');
        $raw = "From: {$sender}\r\n"
            ."To: {$recipient}\r\n"
            ."Subject: MailWeb response {$response['request_id']}\r\n"
            ."MIME-Version: 1.0\r\n"
            ."Content-Type: application/mailweb+json; charset=utf-8\r\n"
            ."Content-Transfer-Encoding: 8bit\r\n\r\n"
            .$json;
        $envelope = new Envelope(new Address($sender), [new Address($recipient)]);
        (new Mailer(new EsmtpTransport($host, $port, false)))->send(new RawMessage($raw), $envelope);
    }

    private function markRead(string $api, string $id): void
    {
        Http::timeout(3)->put("{$api}/api/v1/messages", [
            'IDs' => [$id],
            'Read' => true,
        ])->throw();
    }
}
