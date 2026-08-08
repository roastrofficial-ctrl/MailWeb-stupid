<?php

namespace App\MailWeb;

use Illuminate\Support\Facades\Validator;
use RuntimeException;

final readonly class MailWebRequest
{
    public function __construct(
        public string $version,
        public string $id,
        public string $method,
        public string $uri,
        public array $headers,
        public array $body,
        public string $path,
        public array $query,
    ) {}

    public static function fromArray(array $message): self
    {
        $validated = Validator::make($message, [
            'mailweb' => ['required', 'in:0.1,0.2'],
            'id' => ['required', 'string', 'regex:/^[0-9A-HJKMNP-TV-Z]{26}$/'],
            'method' => ['required', 'in:GET,POST'],
            'uri' => ['required', 'string', 'starts_with:mailweb://', 'max:2048'],
            'headers' => ['present', 'array'],
            'headers.*' => ['string'],
            'body' => ['sometimes', 'array'],
        ])->validate();

        if (array_diff(array_keys($message), ['mailweb', 'id', 'method', 'uri', 'headers', 'body']) !== []) {
            throw new RuntimeException('MailWebRequest contains unknown fields');
        }
        if ($validated['mailweb'] === '0.1' && ($validated['method'] !== 'GET' || isset($message['body']))) {
            throw new RuntimeException('MailWeb 0.1 supports bodyless GET requests only');
        }
        if ($validated['method'] === 'GET' && isset($message['body'])) {
            throw new RuntimeException('GET requests must not contain a body');
        }

        $query = [];
        parse_str((string) parse_url($validated['uri'], PHP_URL_QUERY), $query);

        return new self(
            $validated['mailweb'],
            $validated['id'],
            $validated['method'],
            $validated['uri'],
            $validated['headers'],
            $validated['body'] ?? [],
            parse_url($validated['uri'], PHP_URL_PATH) ?: '/',
            $query,
        );
    }
}
