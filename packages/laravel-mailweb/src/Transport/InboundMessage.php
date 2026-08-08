<?php

namespace MailWeb\Laravel\Transport;

final readonly class InboundMessage
{
    public function __construct(public string $id, public string $sender, public string $raw) {}
}
