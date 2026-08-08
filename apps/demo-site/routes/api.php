<?php

use Illuminate\Http\Request;
use Illuminate\Support\Facades\Route;
Route::post('/mailweb/messages', function (Request $request) {
    $message = $request->validate([
        'mailweb' => ['required', 'in:0.1'],
        'id' => ['required', 'string', 'regex:/^[0-9A-HJKMNP-TV-Z]{26}$/'],
        'method' => ['required', 'in:GET'],
        'uri' => ['required', 'string', 'starts_with:mailweb://', 'max:2048'],
        'headers' => ['present', 'array'],
    ]);

    $path = parse_url($message['uri'], PHP_URL_PATH) ?: '/';
    $status = $path === '/' ? 200 : 404;
    $title = $status === 200 ? 'Hello from MailWeb' : 'Document not found';
    $body = $status === 200 ? [
        ['type' => 'heading', 'level' => 1, 'text' => 'Hello, Internet'],
        ['type' => 'paragraph', 'text' => 'This page arrived as a private message.'],
        ['type' => 'link', 'label' => 'About this nonsense', 'href' => '/about'],
        ['type' => 'button', 'label' => 'Request it again', 'href' => '/'],
    ] : [
        ['type' => 'heading', 'level' => 1, 'text' => 'Document not found'],
        ['type' => 'paragraph', 'text' => "The publisher has no document for {$path}."],
        ['type' => 'link', 'label' => 'Return home', 'href' => '/'],
    ];

    return response()->json([
        'mailweb' => '0.1',
        'request_id' => $message['id'],
        'status' => $status,
        'document' => ['title' => $title, 'body' => $body],
    ]);
});
