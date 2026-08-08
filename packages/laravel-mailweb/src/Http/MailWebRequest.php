<?php

namespace MailWeb\Laravel\Http;

final readonly class MailWebRequest
{
    /** @param array<string, string> $headers
     *  @param array<string, mixed> $body
     *  @param array<string, mixed> $query
     */
    public function __construct(
        private string $version,
        private string $id,
        private string $requestMethod,
        private string $requestUri,
        private array $headers,
        private array $body,
        private array $query,
        private string $path,
    ) {}

    public function version(): string { return $this->version; }
    public function id(): string { return $this->id; }
    public function method(): string { return $this->requestMethod; }
    public function uri(): string { return $this->requestUri; }
    public function path(): string { return $this->path; }

    public function input(?string $key = null, mixed $default = null): mixed
    {
        $all = $this->all();
        return $key === null ? $all : data_get($all, $key, $default);
    }

    public function query(?string $key = null, mixed $default = null): mixed
    {
        return $key === null ? $this->query : data_get($this->query, $key, $default);
    }

    /** @return array<string, mixed> */
    public function all(): array
    {
        return array_replace($this->query, $this->body);
    }

    public function header(string $name, ?string $default = null): ?string
    {
        foreach ($this->headers as $header => $value) {
            if (strcasecmp($header, $name) === 0) {
                return $value;
            }
        }
        return $default;
    }
}
