package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	transportName := flag.String("transport", envOr("MAILWEB_TRANSPORT", "http"), "transport to use: http or smtp")
	timeout := flag.Duration("timeout", 15*time.Second, "how long to wait for a transport response")
	webUI := flag.Bool("ui", false, "host the graphical Postbox browser")
	listen := flag.String("listen", "127.0.0.1:9847", "loopback address for the graphical UI")
	flag.Parse()
	if (!*webUI && flag.NArg() != 1) || (*webUI && flag.NArg() > 1) {
		fmt.Fprintln(os.Stderr, "usage: postbox [--transport http|smtp] [--ui] [mailweb://publisher/path]")
		os.Exit(2)
	}
	var transport Transport
	switch *transportName {
	case "http":
		transport = NewHTTPTransport(envOr("MAILWEB_PUBLISHER_URL", "http://localhost:8081/mailweb/messages"))
	case "smtp":
		transport = NewSMTPTransport(
			envOr("MAILWEB_SMTP_ADDRESS", "localhost:1025"),
			envOr("MAILWEB_PUBLISHER_MAILBOX", "browse@demo.local"),
			envOr("MAILWEB_MAILPIT_URL", "http://localhost:8025"),
			*timeout,
			os.Stdout,
		)
	default:
		fmt.Fprintf(os.Stderr, "postbox: unknown transport %q\n", *transportName)
		os.Exit(2)
	}
	if *webUI {
		session := NewBrowserSession(transport, *transportName)
		if flag.NArg() == 1 {
			if _, err := session.Navigate(context.Background(), flag.Arg(0)); err != nil {
				fmt.Fprintf(os.Stderr, "postbox: initial navigation failed: %v\n", err)
			}
		}
		if err := RunWebUI(*listen, session); err != nil {
			fmt.Fprintf(os.Stderr, "postbox: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := Browse(context.Background(), transport, flag.Arg(0), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "postbox: %v\n", err)
		os.Exit(1)
	}
}

func Browse(ctx context.Context, transport Transport, initialURI string, input io.Reader, output io.Writer) error {
	session := NewBrowserSession(transport, "terminal")
	session.DisablePrefetch()
	scanner := bufio.NewScanner(input)
	state, err := session.Navigate(ctx, initialURI)
	for {
		if err != nil {
			return fmt.Errorf("navigation failed: %w", err)
		}

		links := RenderTerminal(output, state.Current.Response)
		if len(links) == 0 {
			return nil
		}
		fmt.Fprint(output, "Choose a link number, or q to quit: ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
			return nil
		}
		choice := strings.TrimSpace(scanner.Text())
		if strings.EqualFold(choice, "q") {
			return nil
		}
		index, err := strconv.Atoi(choice)
		if err != nil || index < 1 || index > len(links) {
			fmt.Fprintln(output, "Please enter one of the displayed numbers.")
			continue
		}
		state, err = session.NavigateReference(ctx, links[index-1].Href)
		fmt.Fprintln(output)
	}
}

func resolveMailWebReference(base, reference string) (string, error) {
	baseURI, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	referenceURI, err := url.Parse(reference)
	if err != nil {
		return "", err
	}
	resolved := baseURI.ResolveReference(referenceURI)
	if resolved.Scheme != "mailweb" {
		return "", errors.New("terminal navigation only supports MailWeb links")
	}
	return resolved.String(), nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
