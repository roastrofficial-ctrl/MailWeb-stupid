<?php

return [
    'protocol' => '0.2',
    'routes' => base_path('routes/mailweb.php'),
    'http_endpoint' => '/mailweb/messages',
    'publisher_address' => env('MAILWEB_PUBLISHER_MAILBOX', 'browse@demo.local'),

    'inbound' => [
        'driver' => env('MAILWEB_INBOUND_DRIVER', 'mailpit'),
        'mailpit_url' => env('MAILWEB_MAILPIT_URL', 'http://mailpit:8025'),
        'poll_milliseconds' => (int) env('MAILWEB_POLL_MILLISECONDS', 500),
    ],

    'smtp' => [
        'host' => env('MAILWEB_SMTP_HOST', 'mailpit'),
        'port' => (int) env('MAILWEB_SMTP_PORT', 1025),
    ],
];
