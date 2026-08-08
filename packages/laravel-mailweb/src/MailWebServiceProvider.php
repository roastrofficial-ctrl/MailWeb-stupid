<?php

namespace MailWeb\Laravel;

use Illuminate\Contracts\Foundation\Application;
use Illuminate\Http\Client\Factory as Http;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Route;
use Illuminate\Support\ServiceProvider;
use MailWeb\Laravel\Console\ListenCommand;
use MailWeb\Laravel\Protocol\ProtocolException;
use MailWeb\Laravel\Protocol\Publisher;
use MailWeb\Laravel\Routing\MailWebRouter;
use MailWeb\Laravel\Transport\Inbox;
use MailWeb\Laravel\Transport\MailpitInbox;
use MailWeb\Laravel\Transport\SmtpResponder;
use RuntimeException;

final class MailWebServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->mergeConfigFrom(__DIR__.'/../config/mailweb.php', 'mailweb');
        $this->app->singleton(MailWebRouter::class);
        $this->app->singleton('mailweb', fn (Application $app) => new MailWebManager($app->make(MailWebRouter::class)));
        $this->app->bind(Inbox::class, function (Application $app): Inbox {
            if (config('mailweb.inbound.driver') !== 'mailpit') {
                throw new RuntimeException('Unsupported MailWeb inbound driver. Install or bind an Inbox implementation.');
            }
            return new MailpitInbox($app->make(Http::class), config('mailweb.inbound.mailpit_url'), config('mailweb.publisher_address'));
        });
        $this->app->bind(SmtpResponder::class, fn () => new SmtpResponder(
            config('mailweb.smtp.host'), (int) config('mailweb.smtp.port'), config('mailweb.publisher_address'),
        ));
    }

    public function boot(Publisher $publisher): void
    {
        $this->publishes([__DIR__.'/../config/mailweb.php' => config_path('mailweb.php')], 'mailweb-config');
        if ($this->app->runningInConsole()) { $this->commands([ListenCommand::class]); }

        $routes = config('mailweb.routes');
        if (is_string($routes) && is_file($routes)) { require $routes; }

        $endpoint = config('mailweb.http_endpoint');
        if (is_string($endpoint) && $endpoint !== '') {
            Route::post($endpoint, function (Request $request) use ($publisher) {
                try {
					$payload = $request->getContent();
					if (strlen($payload) > 1 << 20) {
						return response()->json(['error' => 'MailWebRequest exceeds the 1 MiB carrier limit.'], 413);
					}
                    return response()->json($publisher->handle($payload));
                } catch (ProtocolException $error) {
                    return response()->json(['error' => $error->getMessage()], 422);
                }
            });
        }
    }
}
