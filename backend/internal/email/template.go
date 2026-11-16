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

var ErrUnknownTemplate = errors.New("unknown email template")

type Rendered struct {
	Subject string
	Text    string
	HTML    string
}

type Renderer interface {
	Render(purpose Purpose, data Data) (Rendered, error)
}

type templateSet struct {
	subject *texttemplate.Template
	text    *texttemplate.Template
	html    *htmltemplate.Template
}

type EmbeddedRenderer struct {
	templates map[Purpose]templateSet
}

func NewRenderer() (*EmbeddedRenderer, error) {
	renderer := &EmbeddedRenderer{templates: make(map[Purpose]templateSet, 4)}
	for _, purpose := range []Purpose{PurposeVerifyEmail, PurposeLoginOTP, PurposeAccountExists, PurposeDeleteAccount} {
		set, err := parseTemplateSet(purpose)
		if err != nil {
			return nil, err
		}
		renderer.templates[purpose] = set
	}
	return renderer, nil
}

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
