<?php

use Illuminate\Support\Facades\Artisan;
use App\MailWeb\InboxConsumer;

Artisan::command('mailweb:health', function () {
    $this->info('ok');
})->purpose('Verify that the Laravel application can boot');

Artisan::command('mailweb:consume', function () {
    app(InboxConsumer::class)->consume($this);
})->purpose('Consume MailWeb requests from the development mailbox');
