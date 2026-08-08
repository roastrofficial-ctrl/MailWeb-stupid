<?php

use Illuminate\Http\Request;
use Illuminate\Support\Facades\Route;
use App\MailWeb\Publisher;
Route::post('/mailweb/messages', function (Request $request) {
    $message = $request->validate([
        'mailweb' => ['required', 'in:0.1'],
        'id' => ['required', 'string', 'regex:/^[0-9A-HJKMNP-TV-Z]{26}$/'],
        'method' => ['required', 'in:GET'],
        'uri' => ['required', 'string', 'starts_with:mailweb://', 'max:2048'],
        'headers' => ['present', 'array'],
    ]);

    return response()->json(app(Publisher::class)->respond($message));
});
