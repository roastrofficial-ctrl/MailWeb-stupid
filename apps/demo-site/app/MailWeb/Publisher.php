<?php

namespace App\MailWeb;

final class Publisher
{
    public function respond(MailWebRequest $request): array
    {
        $path = $request->path;
        $status = in_array($path, ['/', '/proof', '/why', '/hello'], true) ? 200 : 404;
		$submittedName = $request->body['name'] ?? '';
		$name = is_string($submittedName) ? trim(mb_substr($submittedName, 0, 80)) : '';
        $title = match ($path) {
            '/' => 'Dear Internet',
            '/proof' => 'You just clicked a link',
            '/why' => 'The Internet Is Not HTTP',
			'/hello' => $request->method === 'POST' ? 'A personal reply' : 'An introduction',
            default => 'Document not found',
        };
        $body = match ($path) {
            '/' => [
                ['type' => 'heading', 'level' => 1, 'text' => 'Dear Internet'],
                ['type' => 'paragraph', 'text' => 'This looks like a website.'],
                ['type' => 'paragraph', 'text' => "It isn't."],
                ['type' => 'paragraph', 'text' => "Everything you're reading arrived as a private message."],
                ['type' => 'button', 'label' => 'Prove it', 'href' => '/proof'],
				['type' => 'link', 'label' => 'Correspond with us', 'href' => '/hello'],
				['type' => 'link', 'label' => 'Read the proof', 'href' => '/proof'],
				['type' => 'link', 'label' => 'Why this nonsense?', 'href' => '/why'],
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
			'/hello' => $request->method === 'POST' ? [
				['type' => 'heading', 'level' => 1, 'text' => 'Dear '.($name !== '' ? $name : 'correspondent').','],
				['type' => 'paragraph', 'text' => 'Lovely to correspond.'],
				['type' => 'paragraph', 'text' => 'Your introduction travelled here as a private POST, and this reply travelled back the same way.'],
				['type' => 'link', 'label' => 'Write another letter', 'href' => '/hello'],
			] : [
				['type' => 'heading', 'level' => 1, 'text' => 'Dear Internet'],
				['type' => 'paragraph', 'text' => "What's your name?"],
				['type' => 'form', 'method' => 'POST', 'action' => '/hello', 'fields' => [[
					'name' => 'name', 'type' => 'text', 'label' => 'What should we call you?',
					'placeholder' => 'Your name', 'required' => true,
				]], 'submit' => 'Send by post'],
				['type' => 'link', 'label' => 'Return home', 'href' => '/'],
			],
            default => [
                ['type' => 'heading', 'level' => 1, 'text' => 'Document not found'],
                ['type' => 'paragraph', 'text' => "The publisher has no document for {$path}."],
                ['type' => 'link', 'label' => 'Return home', 'href' => '/'],
            ],
        };

        return [
			'mailweb' => $request->version,
			'request_id' => $request->id,
            'status' => $status,
            'document' => ['title' => $title, 'body' => $body],
        ];
    }
}
