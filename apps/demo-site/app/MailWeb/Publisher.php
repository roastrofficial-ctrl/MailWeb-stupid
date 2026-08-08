<?php

namespace App\MailWeb;

final class Publisher
{
    public function respond(array $request): array
    {
        $path = parse_url($request['uri'], PHP_URL_PATH) ?: '/';
        $status = in_array($path, ['/', '/proof', '/why'], true) ? 200 : 404;
        $title = match ($path) {
            '/' => 'Dear Internet',
            '/proof' => 'You just clicked a link',
            '/why' => 'The Internet Is Not HTTP',
            default => 'Document not found',
        };
        $body = match ($path) {
            '/' => [
                ['type' => 'heading', 'level' => 1, 'text' => 'Dear Internet'],
                ['type' => 'paragraph', 'text' => 'This looks like a website.'],
                ['type' => 'paragraph', 'text' => "It isn't."],
                ['type' => 'paragraph', 'text' => "Everything you're reading arrived as a private message."],
                ['type' => 'button', 'label' => 'Prove it', 'href' => '/proof'],
            ],
            '/proof' => [
                ['type' => 'heading', 'level' => 1, 'text' => 'You just clicked a link.'],
                ['type' => 'paragraph', 'text' => 'Your computer mailed this:'],
                ['type' => 'paragraph', 'text' => 'GET mailweb://demo.local/proof'],
                ['type' => 'paragraph', 'text' => 'The publisher replied with another private message.'],
                ['type' => 'paragraph', 'text' => 'And Postbox rendered it.'],
                ['type' => 'button', 'label' => 'Do it again', 'href' => '/proof'],
                ['type' => 'link', 'label' => 'Why?', 'href' => '/why'],
            ],
            '/why' => [
                ['type' => 'heading', 'level' => 1, 'text' => 'The Internet Is Not HTTP.'],
                ['type' => 'paragraph', 'text' => 'HTTP is one transport mechanism.'],
                ['type' => 'paragraph', 'text' => 'Presentation and transport are separate concerns.'],
                ['type' => 'paragraph', 'text' => 'MailWeb is deliberately absurd.'],
                ['type' => 'paragraph', 'text' => "That's the point."],
                ['type' => 'link', 'label' => 'Write home', 'href' => '/'],
            ],
            default => [
                ['type' => 'heading', 'level' => 1, 'text' => 'Document not found'],
                ['type' => 'paragraph', 'text' => "The publisher has no document for {$path}."],
                ['type' => 'link', 'label' => 'Return home', 'href' => '/'],
            ],
        };

        return [
            'mailweb' => '0.1',
            'request_id' => $request['id'],
            'status' => $status,
            'document' => ['title' => $title, 'body' => $body],
        ];
    }
}
