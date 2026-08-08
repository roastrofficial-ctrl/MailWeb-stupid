<?php

namespace MailWeb\Laravel\Console;

use Illuminate\Console\Command;
use MailWeb\Laravel\Protocol\Publisher;
use MailWeb\Laravel\Transport\Inbox;
use MailWeb\Laravel\Transport\MessageParser;
use MailWeb\Laravel\Transport\SmtpResponder;
use Throwable;

final class ListenCommand extends Command
{
    protected $signature = 'mailweb:listen {--once : Process currently available correspondence and exit}';
    protected $description = 'Listen for MailWeb requests and send correlated responses';

    public function handle(Inbox $inbox, MessageParser $parser, Publisher $publisher, SmtpResponder $responder): int
    {
        $this->info('Watching '.config('mailweb.publisher_address').' for MailWeb requests...');
        do {
            try {
                foreach ($inbox->unread() as $message) {
                    $inbox->markRead($message->id);
                    try {
                        $response = $publisher->handle($parser->payload($message->raw));
                        $responder->send($message->sender, $response);
                        $this->line("Replied to {$message->sender} for request {$response['request_id']}");
                    } catch (Throwable $error) {
                        $this->error("Skipped message {$message->id}: {$error->getMessage()}");
                    }
                }
            } catch (Throwable $error) {
                $this->error($error->getMessage());
            }
            if (! $this->option('once')) { usleep(max(50, (int) config('mailweb.inbound.poll_milliseconds')) * 1000); }
        } while (! $this->option('once'));
        return self::SUCCESS;
    }
}
