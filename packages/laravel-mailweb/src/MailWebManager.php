<?php

namespace MailWeb\Laravel;

use MailWeb\Laravel\Documents\Page;
use MailWeb\Laravel\Documents\TextField;
use MailWeb\Laravel\Routing\MailWebRoute;
use MailWeb\Laravel\Routing\MailWebRouter;

final readonly class MailWebManager
{
    public function __construct(private MailWebRouter $router) {}

    public function get(string $uri, callable|array|string $action): MailWebRoute
    {
        return $this->router->add('GET', $uri, $action);
    }

    public function post(string $uri, callable|array|string $action): MailWebRoute
    {
        return $this->router->add('POST', $uri, $action);
    }

    public function page(string $title, int $status = 200): Page
    {
        return new Page($title, $status);
    }

    public function text(string $name, string $label, string $placeholder = '', bool $required = false): TextField
    {
        return new TextField($name, $label, $placeholder, $required);
    }
}
