<?php

namespace MailWeb\Laravel\Documents;

use InvalidArgumentException;
use JsonSerializable;

final readonly class TextField implements JsonSerializable
{
    public function __construct(
        public string $name,
        public string $label,
        public string $placeholder = '',
        public bool $required = false,
    ) {
        if (! preg_match('/^[A-Za-z][A-Za-z0-9_.-]{0,63}$/', $name) || $label === '') {
            throw new InvalidArgumentException('Text fields require a safe name and non-empty label.');
        }
    }

    public function jsonSerialize(): array
    {
        return array_filter([
            'name' => $this->name,
            'type' => 'text',
            'label' => $this->label,
            'placeholder' => $this->placeholder,
            'required' => $this->required,
        ], fn (mixed $value, string $key) => ! (($key === 'placeholder' && $value === '') || ($key === 'required' && $value === false)), ARRAY_FILTER_USE_BOTH);
    }
}
