<?php

namespace App\MailWeb;

final class Publisher
{
    public function respond(array $request): array
    {
        $path = parse_url($request['uri'], PHP_URL_PATH) ?: '/';
        $status = in_array($path, ['/', '/about'], true) ? 200 : 404;
        $title = match ($path) {
            '/' => 'Hello from MailWeb',
            '/about' => 'About MailWeb',
            default => 'Document not found',
        };
        $body = match ($path) {
            '/' => [
                ['type' => 'heading', 'level' => 1, 'text' => 'Hello, Internet'],
                ['type' => 'paragraph', 'text' => 'This page arrived as a private message.'],
                ['type' => 'link', 'label' => 'About this nonsense', 'href' => '/about'],
            ],
            '/about' => [
                ['type' => 'heading', 'level' => 1, 'text' => 'About this nonsense'],
                ['type' => 'paragraph', 'text' => 'MailWeb documents travel as private request and response messages, independently of their transport.'],
                ['type' => 'link', 'label' => 'Back home', 'href' => '/'],
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

