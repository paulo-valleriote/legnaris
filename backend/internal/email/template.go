package email

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	htmltemplate "html/template"
	texttemplate "text/template"
)

//go:embed templates/*
var templateFS embed.FS

// ErrUnknownTemplate reports a purpose that selects no embedded template; the
// effect is a permanent failure with no send attempted (EML-11).
//
// It is a plain errors.New sentinel, never apperr: this domain is a non-HTTP
// layer with no status to map (see ErrTransient in sender.go).
var ErrUnknownTemplate = errors.New("unknown email template")

// Rendered is the subject and both bodies produced for one purpose.
type Rendered struct {
	Subject string
	Text    string
	HTML    string
}

// Renderer turns a purpose and its data into a code-free subject and both a
// plain-text and an HTML body carrying the code and its lifetime (EML-11).
type Renderer interface {
	Render(purpose Purpose, data Data) (Rendered, error)
}

// templateSet is one purpose's parsed subject, text and HTML templates. The
// subject and text are parsed with text/template; the HTML body with
// html/template, so the code is escaped there and verbatim in text (EML-11).
type templateSet struct {
	subject *texttemplate.Template
	text    *texttemplate.Template
	html    *htmltemplate.Template
}

// EmbeddedRenderer renders the six embedded per-purpose templates.
type EmbeddedRenderer struct {
	templates map[Purpose]templateSet
}

// NewRenderer parses the embedded templates for every known purpose. Parsing
// happens once at construction so rendering stays pure (EML-11, EML-22).
func NewRenderer() (*EmbeddedRenderer, error) {
	renderer := &EmbeddedRenderer{templates: make(map[Purpose]templateSet, 2)}
	for _, purpose := range []Purpose{PurposeVerifyEmail, PurposeLoginOTP} {
		set, err := parseTemplateSet(purpose)
		if err != nil {
			return nil, err
		}
		renderer.templates[purpose] = set
	}
	return renderer, nil
}

// Render selects the template pair by purpose; an unknown purpose renders
// nothing and returns ErrUnknownTemplate (EML-11).
func (r *EmbeddedRenderer) Render(purpose Purpose, data Data) (Rendered, error) {
	set, ok := r.templates[purpose]
	if !ok {
		return Rendered{}, fmt.Errorf("%w: %q", ErrUnknownTemplate, purpose)
	}

	var subject, text, html bytes.Buffer
	if err := set.subject.Execute(&subject, data); err != nil {
		return Rendered{}, fmt.Errorf("render subject %q: %w", purpose, err)
	}
	if err := set.text.Execute(&text, data); err != nil {
		return Rendered{}, fmt.Errorf("render text %q: %w", purpose, err)
	}
	if err := set.html.Execute(&html, data); err != nil {
		return Rendered{}, fmt.Errorf("render html %q: %w", purpose, err)
	}

	return Rendered{Subject: subject.String(), Text: text.String(), HTML: html.String()}, nil
}

// parseTemplateSet reads and parses one purpose's three embedded files.
func parseTemplateSet(purpose Purpose) (templateSet, error) {
	subject, err := templateFS.ReadFile(templatePath(purpose, "subject.txt"))
	if err != nil {
		return templateSet{}, fmt.Errorf("read %s subject: %w", purpose, err)
	}
	text, err := templateFS.ReadFile(templatePath(purpose, "body.txt"))
	if err != nil {
		return templateSet{}, fmt.Errorf("read %s text: %w", purpose, err)
	}
	html, err := templateFS.ReadFile(templatePath(purpose, "body.html"))
	if err != nil {
		return templateSet{}, fmt.Errorf("read %s html: %w", purpose, err)
	}

	parsedSubject, err := texttemplate.New("subject").Parse(string(subject))
	if err != nil {
		return templateSet{}, fmt.Errorf("parse %s subject: %w", purpose, err)
	}
	parsedText, err := texttemplate.New("text").Parse(string(text))
	if err != nil {
		return templateSet{}, fmt.Errorf("parse %s text: %w", purpose, err)
	}
	parsedHTML, err := htmltemplate.New("html").Parse(string(html))
	if err != nil {
		return templateSet{}, fmt.Errorf("parse %s html: %w", purpose, err)
	}

	return templateSet{subject: parsedSubject, text: parsedText, html: parsedHTML}, nil
}

func templatePath(purpose Purpose, suffix string) string {
	return fmt.Sprintf("templates/%s.%s", purpose, suffix)
}
