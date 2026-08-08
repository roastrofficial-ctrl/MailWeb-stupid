<?php

namespace MailWeb\Laravel\Routing;

final readonly class MailWebRoute
{
    /** @param callable|array|string $action */
    public function __construct(public string $method, public string $uri, public mixed $action) {}

    /** @return array<string, string>|null */
    public function match(string $method, string $path): ?array
    {
        if ($method !== $this->method) {
            return null;
        }
        $names = [];
        $quoted = preg_quote($this->uri, '#');
        $pattern = preg_replace_callback('/\\\\\{([A-Za-z_][A-Za-z0-9_]*)\\\\\}/', function (array $match) use (&$names): string {
            $names[] = $match[1];
            return '([^/]+)';
        }, $quoted);
        if (! is_string($pattern) || ! preg_match('#^'.$pattern.'$#', $path, $matches)) {
            return null;
        }
        array_shift($matches);
        return array_combine($names, array_map('urldecode', $matches)) ?: [];
    }
}
