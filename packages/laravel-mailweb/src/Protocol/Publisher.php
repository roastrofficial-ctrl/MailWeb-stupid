<?php

namespace MailWeb\Laravel\Protocol;

use MailWeb\Laravel\Routing\MailWebRouter;

final readonly class Publisher
{
    public function __construct(private ProtocolCodec $codec, private MailWebRouter $router) {}

    /** @return array<string, mixed> */
    public function handle(string|array $payload): array
    {
        $request = $this->codec->decode($payload);
        return $this->codec->response($request, $this->router->dispatch($request));
    }
}
