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
	enclosures := map[string]Enclosure{}
	for _, enclosure := range response.Enclosures {
		enclosures[enclosure.Digest] = enclosure
	}
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
		case "nav":
			fmt.Fprintln(writer, node.Label)
			for _, item := range node.Items {
				navigation = append(navigation, Navigation{Label: item.Label, Href: item.Href})
				fmt.Fprintf(writer, "[%d] %s\n", len(navigation), item.Label)
			}
			fmt.Fprintln(writer)
		case "image":
			alt := strings.TrimSpace(node.Alt)
			if alt == "" {
				alt = "Image"
			}
			fmt.Fprintf(writer, "[%s]\n\n", alt)
		case "attachment":
			file := enclosures[node.Digest]
			fmt.Fprintf(writer, "[Attachment: %s · %s · %d bytes]\n\n", file.Filename, file.MediaType, file.Size)
		case "client_action":
			fmt.Fprintf(writer, "[%s — open graphical Postbox security UI]\n\n", node.Label)
		}
	}
	return navigation
}
