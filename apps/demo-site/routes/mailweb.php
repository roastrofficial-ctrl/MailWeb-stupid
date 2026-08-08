<?php

use MailWeb\Laravel\Facades\MailWeb;
use MailWeb\Laravel\Http\MailWebRequest;

MailWeb::get('/', fn () => MailWeb::page('Dear Internet')
	->presentation('#315C45', '#FFFDF8', '#17231C', '#F3EFE5', 'editorial', 'spacious', 'soft')
    ->heading('Dear Internet', variant: 'display')
    ->paragraph('This looks like a website. It is actually private correspondence.')
    ->paragraph('Postbox did not HTTP GET this page from a web server. It wrote to a private email address and waited for this document to arrive in the reply.')
    ->paragraph('Every heading, paragraph, link and button below was sealed inside that private message, then safely rendered by your MailWeb browser.')
    ->button('Prove it was posted', '/proof', 'prominent')
    ->link('Correspond with us', '/hello')
    ->link('Inspect the private exchange', '/proof')
    ->link('Why browse by correspondence?', '/why'));

MailWeb::get('/proof', fn () => MailWeb::page('You just clicked a link')
    ->heading('You just mailed a link.')
    ->paragraph('That click did not navigate your HTTP browser to a web server. Postbox privately mailed this request:')
    ->paragraph('GET mailweb://demo.local/proof')
    ->paragraph('The demo MailWeb configured application opened the correspondence, routed the enclosed MailWeb request, and privately emailed this document back to your personal Postbox address.')
    ->paragraph('Postbox opened the reply and rendered it as the page you are reading now. The Message Inspector shows both letters and their matching request ID.')
    ->button('Mail the same page again', '/proof')
    ->link('Why turn browsing into private correspondence?', '/why'));

MailWeb::get('/why', fn () => MailWeb::page('The Internet Is Not HTTP')
    ->heading('The Internet Is Not HTTP.')
    ->paragraph('A web-style document does not have to arrive as an HTTP response. HTTP is one delivery mechanism, not the document itself.')
    ->paragraph('MailWeb separates presentation from transport, then makes the transport conspicuously personal: every page view becomes private correspondence between you and an application.')
    ->paragraph('This is not a claim that email is a sensible replacement for the Web. It is an absurd demonstration that the Web and HTTP are not the same idea.')
    ->paragraph('You are browsing a demo MailWeb enabled Laravel application by exchanging private messages with it. That is the joke. It is also genuinely happening.')
    ->link('Write privately to the home page', '/'));

MailWeb::get('/hello', fn () => MailWeb::page('An introduction')
    ->heading('A private introduction')
    ->paragraph('This form is not about to POST over HTTP. Fill it in and Postbox will place your answer inside a private MailWeb message addressed to the demo MailWeb enabled Laravel application.')
    ->paragraph("What name should we put in the letter?")
    ->form('POST', '/hello', [
        MailWeb::text(name: 'name', label: 'What should we call you?', placeholder: 'Your name', required: true),
    ], 'Send private correspondence')
    ->link('Return to your earlier correspondence', '/'));

MailWeb::post('/hello', function (MailWebRequest $request) {
    $submitted = $request->input('name', '');
    $name = is_string($submitted) ? trim(mb_substr($submitted, 0, 80)) : '';
    $name = $name !== '' ? $name : 'correspondent';

    return MailWeb::page('A personal reply')
        ->heading("Dear {$name},")
        ->paragraph('Lovely to correspond privately.')
        ->paragraph('Your name arrived inside a POST request carried by private email. The demo MailWeb enabled Laravel application read it, generated this personal document, and emailed the response to your Postbox address.')
        ->paragraph('No ordinary webpage form submission occurred. You sent application data through the post and received a web-style page in the reply.')
        ->link('Write another private letter', '/hello');
});
