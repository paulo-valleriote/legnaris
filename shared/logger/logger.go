package logger

import (
	"context"
	"log/slog"
	"os"
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
}

type Logger struct {
	log *slog.Logger
}

func NewLogger() Logger {
	return Logger{
		log: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

func (l Logger) slog() *slog.Logger {
	return l.log
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

func toArgs(attrs []Attribute) []slog.Attr {
	args := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		args[i] = slog.String(attr.Key, attr.Value)
	}
	return args
}
