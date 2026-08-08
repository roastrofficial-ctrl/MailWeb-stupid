<?php

declare(strict_types=1);

use Illuminate\Contracts\Console\Kernel;
use MailWeb\Laravel\Documents\Page;
use MailWeb\Laravel\Facades\MailWeb;
use MailWeb\Laravel\Http\MailWebRequest;
use MailWeb\Laravel\Protocol\ProtocolCodec;
use MailWeb\Laravel\Protocol\ProtocolException;
use MailWeb\Laravel\Protocol\Publisher;

require $argv[1] ?? dirname(__DIR__).'/vendor/autoload.php';
$app = require $argv[2] ?? dirname(__DIR__).'/tests/fixture/bootstrap.php';
$app->make(Kernel::class)->bootstrap();

$assertions = 0;
$assert = function (bool $condition, string $message) use (&$assertions): void {
    $assertions++;
    if (! $condition) { throw new RuntimeException($message); }
};

$assert(config('mailweb.protocol') === '0.3', 'Package configuration did not load.');
$assert(app()->bound('mailweb'), 'Service provider did not bind the MailWeb manager.');
$commands = array_keys(app(Kernel::class)->all());
$assert(in_array('mailweb:listen', $commands, true), 'Listener command was not registered.');

MailWeb::get('/package-test', fn (MailWebRequest $request) => MailWeb::page('Query')
    ->heading((string) $request->query('q'))
    ->paragraph((string) $request->header('x-letter'))
    ->link('Link', '/next')->button('Button', '/next')->image('https://example.test/image.png', 'Image'));
MailWeb::post('/package-test', fn (MailWebRequest $request) => MailWeb::page('Post')
    ->heading((string) $request->input('name'))
    ->form('POST', '/package-test', [MailWeb::text('name', 'Name', 'Your name', true)], 'Send'));
MailWeb::get('/people/{name}', fn (string $name) => MailWeb::page('Person')->heading($name));

$publisher = app(Publisher::class);
$get = $publisher->handle([
    'mailweb' => '0.3', 'id' => '01J00000000000000000000000', 'method' => 'GET',
    'uri' => 'mailweb://demo.local/package-test?q=internet', 'headers' => ['X-Letter' => 'private'],
]);
$assert($get['request_id'] === '01J00000000000000000000000', 'Response correlation changed.');
$assert($get['status'] === 200 && $get['document']['body'][0]['text'] === 'internet', 'GET routing/query failed.');
$assert($get['document']['body'][1]['text'] === 'private', 'Case-insensitive headers failed.');
$assert(array_column($get['document']['body'], 'type') === ['heading', 'paragraph', 'link', 'button', 'image'], 'Document nodes serialized incorrectly.');

$post = $publisher->handle([
    'mailweb' => '0.3', 'id' => '01J00000000000000000000001', 'method' => 'POST',
    'uri' => 'mailweb://demo.local/package-test', 'headers' => ['content-type' => 'application/json'], 'body' => ['name' => 'Levi'],
]);
$assert($post['document']['body'][0]['text'] === 'Levi', 'POST body/input failed.');
$form = $post['document']['body'][1];
$assert($form['type'] === 'form' && $form['fields'][0]['required'] === true, 'Form/text field serialization failed.');
$person = $publisher->handle([
    'mailweb' => '0.3', 'id' => '01J00000000000000000000004', 'method' => 'GET',
    'uri' => 'mailweb://demo.local/people/Ada%20Lovelace', 'headers' => [],
]);
$assert($person['document']['body'][0]['text'] === 'Ada Lovelace', 'Route parameters failed.');
$legacy = $publisher->handle([
    'mailweb' => '0.1', 'id' => '01J00000000000000000000005', 'method' => 'GET',
    'uri' => 'mailweb://demo.local/package-test', 'headers' => [],
]);
$assert($legacy['mailweb'] === '0.1', 'Compatible MailWeb 0.1 GET handling changed.');

$accepted = new Page('Accepted', 202);
$codec = app(ProtocolCodec::class);
$decodedForStatus = $codec->decode([
    'mailweb' => '0.3', 'id' => '01J00000000000000000000006', 'method' => 'GET',
    'uri' => 'mailweb://demo.local/status', 'headers' => [],
]);
$assert($codec->response($decodedForStatus, $accepted)['status'] === 202, 'Response status serialization failed.');
foreach ([
	['mailweb' => '0.4', 'id' => '01J00000000000000000000002', 'method' => 'GET', 'uri' => 'mailweb://demo.local/', 'headers' => []],
	['mailweb' => '0.3', 'id' => 'bad', 'method' => 'GET', 'uri' => 'mailweb://demo.local/', 'headers' => []],
	['mailweb' => '0.3', 'id' => '01J00000000000000000000003', 'method' => 'GET', 'uri' => 'mailweb://demo.local/', 'headers' => [], 'body' => []],
] as $invalid) {
    try { $codec->decode($invalid); $assert(false, 'Malformed request was accepted.'); }
    catch (ProtocolException) { $assert(true, ''); }
}

fwrite(STDOUT, "laravel-mailweb: {$assertions} assertions passed\n");
