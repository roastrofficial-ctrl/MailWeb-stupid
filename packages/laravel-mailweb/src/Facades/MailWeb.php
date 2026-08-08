<?php

namespace MailWeb\Laravel\Facades;

use Illuminate\Support\Facades\Facade;

/** @method static \MailWeb\Laravel\Routing\MailWebRoute get(string $uri, callable|array|string $action)
 *  @method static \MailWeb\Laravel\Routing\MailWebRoute post(string $uri, callable|array|string $action)
 *  @method static \MailWeb\Laravel\Documents\Page page(string $title, int $status = 200)
 *  @method static \MailWeb\Laravel\Documents\TextField text(string $name, string $label, string $placeholder = '', bool $required = false)
 *  @method static \MailWeb\Laravel\Documents\Template template(string $id, ?callable $definition = null)
 */
final class MailWeb extends Facade
{
    protected static function getFacadeAccessor(): string
    {
        return 'mailweb';
    }
}
