<?php

namespace MailWeb\Laravel\Documents;

use InvalidArgumentException;
use JsonSerializable;

final class Page implements JsonSerializable
{
    /** @var array<int, array<string, mixed>> */
    private array $body = [];
	/** @var array<string, string>|null */
	private ?array $presentation = null;

    public function __construct(public readonly string $title, public readonly int $status = 200)
    {
        if ($status < 100 || $status > 599) {
            throw new InvalidArgumentException('MailWeb status must be between 100 and 599.');
        }
    }

    public function heading(string $text, int $level = 1, string $variant = 'normal'): self
    {
        if ($level < 1 || $level > 6) {
            throw new InvalidArgumentException('Heading level must be between 1 and 6.');
        }
		if (! in_array($variant, ['normal', 'display'], true)) { throw new InvalidArgumentException('Invalid heading variant.'); }
        return $this->node(array_filter(['type' => 'heading', 'level' => $level, 'text' => $text, 'variant' => $variant === 'normal' ? null : $variant]));
    }

    public function paragraph(string $text): self
    {
        return $this->node(['type' => 'paragraph', 'text' => $text]);
    }

    public function link(string $label, string $href): self
    {
        return $this->navigation('link', $label, $href);
    }

    public function button(string $label, string $href, string $variant = 'normal'): self
    {
		if (! in_array($variant, ['normal', 'prominent'], true)) { throw new InvalidArgumentException('Invalid button variant.'); }
		return $this->node(array_filter(['type' => 'button', 'label' => $label, 'href' => $href, 'variant' => $variant === 'normal' ? null : $variant]));
    }

    public function image(string $src, string $alt, string $variant = 'normal'): self
    {
		if (! in_array($variant, ['normal', 'hero'], true)) { throw new InvalidArgumentException('Invalid image variant.'); }
        return $this->node(array_filter(['type' => 'image', 'src' => $src, 'alt' => $alt, 'variant' => $variant === 'normal' ? null : $variant]));
    }

	public function presentation(string $accent, string $background, string $foreground, string $surface, string $typeface = 'system', string $density = 'comfortable', string $corners = 'soft'): self
	{
		foreach ([$accent, $background, $foreground, $surface] as $color) {
			if (! preg_match('/^#[0-9A-Fa-f]{6}$/', $color)) { throw new InvalidArgumentException('Presentation colors must be six-digit hex values.'); }
		}
		if (! in_array($typeface, ['system', 'editorial', 'sans', 'mono'], true) || ! in_array($density, ['compact', 'comfortable', 'spacious'], true) || ! in_array($corners, ['square', 'soft', 'round'], true)) { throw new InvalidArgumentException('Invalid presentation intent.'); }
		$this->presentation = compact('accent', 'background', 'foreground', 'surface', 'typeface', 'density', 'corners');
		return $this;
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
                throw new InvalidArgumentException('MailWeb forms support TextField values only.');
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
		$result = ['title' => $this->title, 'body' => $this->body];
		if ($this->presentation !== null) { $result['presentation'] = $this->presentation; }
		return $result;
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
