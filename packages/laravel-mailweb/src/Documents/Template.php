<?php

namespace MailWeb\Laravel\Documents;

use InvalidArgumentException;
use JsonSerializable;

final readonly class Template implements JsonSerializable
{
    public string $version;

    public function __construct(public string $id, public Page $document)
    {
        if (! preg_match('/^[A-Za-z0-9][A-Za-z0-9._\/-]{0,127}$/', $id)) {
            throw new InvalidArgumentException('Invalid MailWeb template ID.');
        }
        $json = json_encode($document->jsonSerialize(), JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE | JSON_THROW_ON_ERROR);
        $this->version = 'sha256:'.hash('sha256', $json);
    }

    public function jsonSerialize(): array
    {
        return ['id' => $this->id, 'version' => $this->version, 'document' => $this->document->jsonSerialize()];
    }
}
