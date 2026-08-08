<?php

use Illuminate\Support\Facades\Artisan;

Artisan::command('mailweb:health', function () {
    $this->info('ok');
})->purpose('Verify that the Laravel application can boot');

