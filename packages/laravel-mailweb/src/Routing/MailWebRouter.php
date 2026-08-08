<?php

namespace MailWeb\Laravel\Routing;

use Illuminate\Contracts\Container\Container;
use MailWeb\Laravel\Documents\Page;
use MailWeb\Laravel\Http\MailWebRequest;
use RuntimeException;

final class MailWebRouter
{
    /** @var array<int, MailWebRoute> */
    private array $routes = [];

    public function __construct(private readonly Container $container) {}

    public function add(string $method, string $uri, callable|array|string $action): MailWebRoute
    {
        $uri = '/'.ltrim($uri, '/');
        $route = new MailWebRoute(strtoupper($method), $uri, $action);
        $this->routes[] = $route;
        return $route;
    }

    public function dispatch(MailWebRequest $request): Page
    {
        foreach ($this->routes as $route) {
            $parameters = $route->match($request->method(), $request->path());
            if ($parameters === null) {
                continue;
            }
            $result = $this->container->call($route->action, [MailWebRequest::class => $request, 'request' => $request, ...$parameters]);
            if (! $result instanceof Page) {
                throw new RuntimeException('MailWeb route handlers must return a Page.');
            }
            return $result;
        }

        return (new Page('Document not found', 404))
            ->heading('Document not found')
            ->paragraph("The publisher has no document for {$request->path()}.")
            ->link('Return home', '/');
    }
}
