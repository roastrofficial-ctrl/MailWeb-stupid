<?php

namespace MailWeb\Laravel\Documents;

use InvalidArgumentException;
use JsonSerializable;

final class Page implements JsonSerializable
{
    /** @var array<int, array<string, mixed>> */
    private array $body = [];

    public function __construct(public readonly string $title, public readonly int $status = 200)
    {
        if ($status < 100 || $status > 599) {
            throw new InvalidArgumentException('MailWeb status must be between 100 and 599.');
        }
    }

    public function heading(string $text, int $level = 1): self
    {
        if ($level < 1 || $level > 6) {
            throw new InvalidArgumentException('Heading level must be between 1 and 6.');
        }
        return $this->node(['type' => 'heading', 'level' => $level, 'text' => $text]);
    }

    public function paragraph(string $text): self
    {
        return $this->node(['type' => 'paragraph', 'text' => $text]);
    }

    public function link(string $label, string $href): self
    {
        return $this->navigation('link', $label, $href);
    }

    public function button(string $label, string $href): self
    {
        return $this->navigation('button', $label, $href);
    }

    public function image(string $src, string $alt): self
    {
        return $this->node(['type' => 'image', 'src' => $src, 'alt' => $alt]);
    }

    /** @param array<int, TextField> $fields */
    public function form(string $method, string $action, array $fields, string $submit): self
    {
        $method = strtoupper($method);
        if (! in_array($method, ['GET', 'POST'], true) || $action === '' || $submit === '' || $fields === []) {
            throw new InvalidArgumentException('Forms require GET or POST, an action, fields, and a submit label.');
        }
        foreach ($fields as $field) {
            if (! $field instanceof TextField) {
                throw new InvalidArgumentException('MailWeb 0.2 forms support TextField values only.');
            }
        }
        return $this->node([
            'type' => 'form', 'method' => $method, 'action' => $action,
            'fields' => array_map(fn (TextField $field) => $field->jsonSerialize(), $fields),
            'submit' => $submit,
        ]);
    }

    /** @param array<string, mixed> $node */
    public function add(array $node): self
    {
        return $this->node($node);
    }

    public function jsonSerialize(): array
    {
        return ['title' => $this->title, 'body' => $this->body];
    }

    private function navigation(string $type, string $label, string $href): self
    {
        if ($label === '' || $href === '') {
            throw new InvalidArgumentException('Navigation nodes require a label and reference.');
        }
        return $this->node(['type' => $type, 'label' => $label, 'href' => $href]);
    }

    /** @param array<string, mixed> $node */
    private function node(array $node): self
    {
        $this->body[] = $node;
        return $this;
    }
}
