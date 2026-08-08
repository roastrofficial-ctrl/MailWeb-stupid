<?php

use MailWeb\Laravel\Facades\MailWeb;
use MailWeb\Laravel\Http\MailWebRequest;

$stationery = MailWeb::template('dear-internet/site', fn () => MailWeb::page('Dear Internet stationery')
	->presentation('#315C45', '#FFFDF8', '#17231C', '#F3EFE5', 'editorial', 'spacious', 'soft')
	->heading('Dear Internet', variant: 'display')
	->nav('Main navigation', [['Home', '/'], ['About', '/about'], ['Hello', '/hello']])
	->slotPlaceholder('content')
	->paragraph('Sent as private correspondence. Typeset locally using stationery already filed by Postbox.'));

$letter = fn (string $title) => MailWeb::page($title)->template($stationery);
$content = fn (string $title) => MailWeb::page($title);

MailWeb::get('/', fn () => $letter('Dear Internet')->slot('content', $content('Home')
    ->heading('This looks like a website.')
    ->paragraph("It isn't. Every word arrived as private correspondence.")
    ->paragraph('This first reply also enclosed reusable semantic stationery: the identity, navigation and footer surrounding this letter.')
    ->paragraph('Follow another address and the publisher will mail only its new content. Postbox already has the site-shaped paper.')
    ->button('Prove it was posted', '/proof', 'prominent')
    ->link('Why browse by correspondence?', '/why')));

MailWeb::get('/about', fn () => $letter('About this correspondence')->slot('content', $content('About')
	->heading('The page did not mail its navigation again.')
	->paragraph('This reply contains About-specific semantic nodes and a reference to dear-internet/site.')
	->paragraph('Postbox found the matching stationery version on file, inserted this letter into its content slot, and rendered the complete page locally.')
	->link('Return home', '/')));

MailWeb::get('/proof', fn () => $letter('You just clicked a link')->slot('content', $content('Proof')
    ->heading('You just mailed a link.')
    ->paragraph('Postbox privately mailed GET mailweb://demo.local/proof and received this content in the correlated reply.')
    ->paragraph('The familiar header, navigation and footer came from stationery already in your Postbox—not from this page response.')
    ->button('Mail the same page again', '/proof')
    ->link('Why turn browsing into correspondence?', '/why')));

MailWeb::get('/why', fn () => $letter('The Internet Is Not HTTP')->slot('content', $content('Why')
    ->heading('The Internet Is Not HTTP.')
    ->paragraph('A web-style document does not have to arrive as an HTTP response. Presentation, reusable structure and transport are separable concerns.')
    ->paragraph('MailWeb is deliberately absurd. Your computer is filing a correspondent’s stationery so their later private letters can masquerade as pages more efficiently.')
    ->link('Write privately to the home page', '/')));

MailWeb::get('/hello', fn () => $letter('An introduction')->slot('content', $content('Hello')
    ->heading('A private introduction')
    ->paragraph('This form will place your answer inside a private MailWeb POST addressed to the Laravel publisher.')
    ->form('POST', '/hello', [MailWeb::text('name', 'What should we call you?', 'Your name', true)], 'Send private correspondence')));

MailWeb::post('/hello', function (MailWebRequest $request) use ($letter, $content) {
    $submitted = $request->input('name', '');
    $name = is_string($submitted) ? trim(mb_substr($submitted, 0, 80)) : '';
    $name = $name !== '' ? $name : 'correspondent';
    return $letter('A personal reply')->slot('content', $content('Reply')
        ->heading("Dear {$name},")
        ->paragraph('Lovely to correspond privately.')
        ->paragraph('Your name arrived inside an emailed MailWeb POST. This personalised content came back using the same stationery already filed in your Postbox.')
        ->link('Write another private letter', '/hello'));
});
