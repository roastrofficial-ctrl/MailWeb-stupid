<?php

declare(strict_types=1);

use Illuminate\Contracts\Console\Kernel;
use MailWeb\Laravel\Protocol\Publisher;

require __DIR__.'/../vendor/autoload.php';
$app = require __DIR__.'/../bootstrap/app.php';
$app->make(Kernel::class)->bootstrap();

$assertions = 0;
$assert = function (bool $condition, string $message) use (&$assertions): void {
    $assertions++;
    if (! $condition) { throw new RuntimeException($message); }
};
$publisher = app(Publisher::class);
$request = fn (string $id, string $method, string $uri, ?array $body = null) => array_filter([
    'mailweb' => '0.4', 'id' => $id, 'method' => $method, 'uri' => $uri,
    'headers' => $method === 'POST' ? ['content-type' => 'application/json'] : [], 'body' => $body,
], fn (mixed $value, string $key) => ! ($key === 'body' && $value === null), ARRAY_FILTER_USE_BOTH);

$home = $publisher->handle($request('01J00000000000000000000010', 'GET', 'mailweb://demo.local/'));
$assert($home['document']['title'] === 'Dear Internet', 'Packaged home route failed.');
$assert(isset($home['templates'][0]) && $home['templates'][0]['document']['presentation']['typeface'] === 'editorial', 'Stationery Presentation Intent was not serialized.');
$legacyHomeRequest = $request('01J00000000000000000000013', 'GET', 'mailweb://demo.local/');
$legacyHomeRequest['mailweb'] = '0.2';
$legacyHome = $publisher->handle($legacyHomeRequest);
$assert(! isset($legacyHome['document']['presentation']) && ! isset($legacyHome['document']['body'][0]['variant']), 'Presentation was not safely downgraded for MailWeb 0.2.');
$assert($home['document']['template'] === 'dear-internet/site' && isset($home['templates'][0]), 'Home did not deliver stationery.');

$form = $publisher->handle($request('01J00000000000000000000011', 'GET', 'mailweb://demo.local/hello'));
$formNodes = array_values(array_filter($form['document']['slots']['content'], fn (array $node) => $node['type'] === 'form'));
$assert(count($formNodes) === 1, 'Packaged form route failed.');

$reply = $publisher->handle($request('01J00000000000000000000012', 'POST', 'mailweb://demo.local/hello', ['name' => 'Levi']));
$assert($reply['request_id'] === '01J00000000000000000000012', 'Demo response correlation failed.');
$assert($reply['document']['slots']['content'][0]['text'] === 'Dear Levi,', 'Dynamic packaged POST failed.');

$packagePath = (new ReflectionClass(Publisher::class))->getFileName();
$assert(is_string($packagePath) && str_contains($packagePath, 'laravel-mailweb'), 'Demo is not loading Publisher from the package.');

fwrite(STDOUT, "demo package integration: {$assertions} assertions passed\n");
