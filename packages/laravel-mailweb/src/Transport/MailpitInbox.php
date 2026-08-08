<?php

namespace MailWeb\Laravel\Transport;

use Illuminate\Http\Client\Factory as Http;
use RuntimeException;

final readonly class MailpitInbox implements Inbox
{
    public function __construct(private Http $http, private string $api, private string $mailbox) {}

    public function unread(): array
    {
        $summaries = $this->http->timeout(3)->get($this->url('/api/v1/search'), [
            'query' => "to:{$this->mailbox} is:unread", 'limit' => 20,
        ])->throw()->json('messages', []);
        $messages = [];
        foreach (array_reverse($summaries) as $summary) {
            $id = $summary['ID'] ?? null;
            if (! is_string($id)) { continue; }
            $message = $this->http->timeout(3)->get($this->url("/api/v1/message/{$id}"))->throw()->json();
            $sender = $message['From']['Address'] ?? null;
            if (! is_string($sender) || $sender === '') {
                throw new RuntimeException("Inbound message {$id} has no sender mailbox.");
            }
            $raw = $this->http->timeout(3)->get($this->url("/api/v1/message/{$id}/raw"))->throw()->body();
            $messages[] = new InboundMessage($id, $sender, $raw);
        }
        return $messages;
    }

    public function markRead(string $id): void
    {
        $this->http->timeout(3)->put($this->url('/api/v1/messages'), ['IDs' => [$id], 'Read' => true])->throw();
    }

    private function url(string $path): string { return rtrim($this->api, '/').$path; }
}
