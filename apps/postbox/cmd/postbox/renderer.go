package main

import (
	"fmt"
	"io"
	"strings"
)

type Navigation struct {
	Label string
	Href  string
}

func RenderTerminal(writer io.Writer, response MailWebResponse) []Navigation {
	fmt.Fprintln(writer, "MAILWEB")
	fmt.Fprintln(writer, "-------")
	fmt.Fprintln(writer)

	var navigation []Navigation
	for _, node := range response.Document.Body {
		switch node.Type {
		case "heading":
			fmt.Fprintln(writer, node.Text)
			fmt.Fprintln(writer)
		case "paragraph":
			fmt.Fprintln(writer, node.Text)
			fmt.Fprintln(writer)
		case "link", "button":
			navigation = append(navigation, Navigation{Label: node.Label, Href: node.Href})
			fmt.Fprintf(writer, "[%d] %s\n\n", len(navigation), node.Label)
		case "image":
			alt := strings.TrimSpace(node.Alt)
			if alt == "" {
				alt = "Image"
			}
			fmt.Fprintf(writer, "[%s]\n\n", alt)
		}
	}
	return navigation
}
