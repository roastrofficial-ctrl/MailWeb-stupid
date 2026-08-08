<?php

namespace MailWeb\Laravel\Documents;

use InvalidArgumentException;
use JsonSerializable;

final readonly class Enclosure implements JsonSerializable
{
    public string $filename; public string $mediaType; public int $size; public string $digest; public string $content;
    public function __construct(public string $id, string $path, ?string $filename = null)
    {
        if (! preg_match('/^[A-Za-z][A-Za-z0-9_.-]{0,63}$/', $id) || ! is_file($path)) { throw new InvalidArgumentException('Enclosure requires a valid ID and readable file.'); }
        $bytes = file_get_contents($path); if ($bytes === false || strlen($bytes) > 2 << 20) { throw new InvalidArgumentException('Enclosure exceeds the 2 MiB development limit.'); }
        $requestedFilename = $filename ?? basename($path); $this->filename = basename($requestedFilename); if ($this->filename === '' || $requestedFilename !== $this->filename || str_contains($this->filename, "\0")) { throw new InvalidArgumentException('Unsafe enclosure filename.'); }
        $detected = (new \finfo(FILEINFO_MIME_TYPE))->buffer($bytes) ?: 'application/octet-stream';
        if (! in_array($detected, ['image/png', 'image/jpeg', 'image/webp', 'application/pdf', 'text/plain'], true)) { throw new InvalidArgumentException("Unsupported enclosure media type {$detected}."); }
        $this->mediaType = $detected; $this->size = strlen($bytes); $this->digest = 'sha256:'.hash('sha256', $bytes); $this->content = base64_encode($bytes);
    }
    public function jsonSerialize(): array { return ['id' => $this->id, 'filename' => $this->filename, 'media_type' => $this->mediaType, 'size' => $this->size, 'digest' => $this->digest, 'content' => $this->content]; }
}
