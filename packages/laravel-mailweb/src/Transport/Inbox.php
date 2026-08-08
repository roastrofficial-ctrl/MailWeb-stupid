<?php

namespace MailWeb\Laravel\Transport;

interface Inbox
{
    /** @return array<int, InboundMessage> */
    public function unread(): array;
    public function markRead(string $id): void;
}
