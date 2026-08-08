<?php

use Illuminate\Http\Request;
use Illuminate\Support\Facades\Route;
use App\MailWeb\Publisher;
use App\MailWeb\MailWebRequest;
Route::post('/mailweb/messages', function (Request $request) {
	$message = MailWebRequest::fromArray($request->json()->all());
	return response()->json(app(Publisher::class)->respond($message));
});
