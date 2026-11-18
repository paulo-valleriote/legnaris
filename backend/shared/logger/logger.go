package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"unicode/utf8"
)

type Attribute struct {
	Key   string
	Value string
}

type StructuredLogger interface {
	Info(ctx context.Context, msg string, args ...Attribute)
	Debug(ctx context.Context, msg string, args ...Attribute)
	Warn(ctx context.Context, msg string, args ...Attribute)
	Error(ctx context.Context, msg string, args ...Attribute)
	Fatal(ctx context.Context, msg string, args ...Attribute)
	With(key string, value string) StructuredLogger
}

type Logger struct {
	log *slog.Logger
}

func NewLogger(level slog.Level) Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: redactSecrets,
	})
	l := slog.New(handler)

	return Logger{
		log: l,
	}
}

func (l Logger) Info(ctx context.Context, msg string, args ...Attribute) {
	l.log.LogAttrs(ctx, slog.LevelInfo, msg, toArgs(args)...)
}

func (l Logger) Debug(ctx context.Context, msg string, args ...Attribute) {
	l.log.LogAttrs(ctx, slog.LevelDebug, msg, toArgs(args)...)
}

func (l Logger) Warn(ctx context.Context, msg string, args ...Attribute) {
	l.log.LogAttrs(ctx, slog.LevelWarn, msg, toArgs(args)...)
}

func (l Logger) Error(ctx context.Context, msg string, args ...Attribute) {
	l.log.LogAttrs(ctx, slog.LevelError, msg, toArgs(args)...)
}

func (l Logger) Fatal(ctx context.Context, msg string, args ...Attribute) {
	l.log.LogAttrs(ctx, slog.LevelError, msg, toArgs(args)...)
	os.Exit(1)
}

func (l Logger) With(key string, value string) StructuredLogger {
	return Logger{
		log: l.log.With(slog.String(key, value)),
	}
}

var redactedKeys = []string{
	"password", "token", "_cookie", "cookie_", "api_key", "_hash", "hash_",
	"document_number", "cpf", "cnpj", "email", "user_email",
	"__host-legnaris_refresh", "__host-legnaris_csrf",
}

var tokenCountKeys = map[string]bool{
	"prompt_tokens":     true,
	"completion_tokens": true,
	"total_tokens":      true,
	"reasoning_tokens":  true,
}

func redactSecrets(groups []string, a slog.Attr) slog.Attr {
	loweredKey := strings.ToLower(a.Key)
	if tokenCountKeys[loweredKey] {
		return a
	}
	for _, key := range redactedKeys {
		if strings.Contains(loweredKey, key) {
			return slog.String(a.Key, "[REDACTED]")
		}
	}
	return a
}

func toArgs(attrs []Attribute) []slog.Attr {
	args := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		args[i] = slog.String(attr.Key, attr.Value)
	}
	return args
}

func Truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	if limit <= 0 {
		return ""
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut]
}
