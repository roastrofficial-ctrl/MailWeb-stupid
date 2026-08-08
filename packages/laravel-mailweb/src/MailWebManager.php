<?php

namespace MailWeb\Laravel;

use MailWeb\Laravel\Documents\Page;
use MailWeb\Laravel\Documents\TextField;
use MailWeb\Laravel\Documents\Template;
use MailWeb\Laravel\Documents\Enclosure;
use MailWeb\Laravel\Routing\MailWebRoute;
use MailWeb\Laravel\Routing\MailWebRouter;

final class MailWebManager
{
	/** @var array<string, Template> */
	private array $templates = [];
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

	public function template(string $id, ?callable $definition = null): Template
	{
		if ($definition !== null) { $page = $definition(); if (! $page instanceof Page) { throw new \InvalidArgumentException('Template definitions must return a MailWeb Page.'); } return $this->templates[$id] = new Template($id, $page); }
		if (! isset($this->templates[$id])) { throw new \InvalidArgumentException("Unknown MailWeb template {$id}."); }
		return $this->templates[$id];
	}

	public function enclosure(string $path, string $id, ?string $filename = null): Enclosure { return new Enclosure($id, $path, $filename); }
}
