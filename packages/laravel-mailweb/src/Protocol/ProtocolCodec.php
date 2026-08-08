<?php

namespace MailWeb\Laravel\Protocol;

use Illuminate\Contracts\Validation\Factory as ValidationFactory;
use Illuminate\Validation\ValidationException;
use JsonException;
use MailWeb\Laravel\Documents\Page;
use MailWeb\Laravel\Http\MailWebRequest;

final readonly class ProtocolCodec
{
    public function __construct(private ValidationFactory $validator) {}

    public function decode(string|array $payload): MailWebRequest
    {
        try {
            $message = is_string($payload) ? json_decode($payload, true, flags: JSON_THROW_ON_ERROR) : $payload;
        } catch (JsonException $error) {
            throw new ProtocolException('Invalid MailWebRequest JSON.', previous: $error);
        }
        if (! is_array($message) || array_is_list($message)) {
            throw new ProtocolException('MailWebRequest must be a JSON object.');
        }
        try {
            $validated = $this->validator->make($message, [
                'mailweb' => ['required', 'in:0.1,0.2,0.3'],
                'id' => ['required', 'string', 'regex:/^[0-9A-HJKMNP-TV-Z]{26}$/'],
                'method' => ['required', 'in:GET,POST'],
                'uri' => ['required', 'string', 'starts_with:mailweb://', 'max:2048'],
                'headers' => ['present', 'array'],
                'headers.*' => ['string'],
                'body' => ['sometimes', 'array'],
            ])->validate();
        } catch (ValidationException $error) {
            throw new ProtocolException('Malformed MailWebRequest: '.$error->validator->errors()->first(), previous: $error);
        }
        if (array_diff(array_keys($message), ['mailweb', 'id', 'method', 'uri', 'headers', 'body']) !== []) {
            throw new ProtocolException('MailWebRequest contains unknown fields.');
        }
        if ($validated['mailweb'] === '0.1' && ($validated['method'] !== 'GET' || array_key_exists('body', $message))) {
            throw new ProtocolException('MailWeb 0.1 supports bodyless GET requests only.');
        }
        if ($validated['method'] === 'GET' && array_key_exists('body', $message)) {
            throw new ProtocolException('GET requests must not contain a body.');
        }
        if ($validated['method'] === 'POST' && ! array_key_exists('body', $message)) {
            throw new ProtocolException('POST requests require a JSON body.');
        }
        $query = [];
        parse_str((string) parse_url($validated['uri'], PHP_URL_QUERY), $query);

        return new MailWebRequest(
            $validated['mailweb'], $validated['id'], $validated['method'], $validated['uri'], $validated['headers'],
            $validated['body'] ?? [], $query, parse_url($validated['uri'], PHP_URL_PATH) ?: '/',
        );
    }

    /** @return array<string, mixed> */
    public function response(MailWebRequest $request, Page $page): array
    {
		$document = $page->jsonSerialize();
		if ($request->version() !== '0.3') {
			unset($document['presentation']);
			foreach ($document['body'] as &$node) { unset($node['variant']); }
			unset($node);
		}
        return [
            'mailweb' => $request->version(),
            'request_id' => $request->id(),
            'status' => $page->status,
			'document' => $document,
        ];
    }
}
