package log

import (
	"context"
	"log/slog"
)

// Context-aware log wrappers. Use these instead of slog directly so trace IDs
// and other request-scoped values can be extracted from ctx in one place later.

func InfoWithContext(ctx context.Context, msg string, args ...any) {
	slog.InfoContext(ctx, msg, args...)
}

func ErrorWithContext(ctx context.Context, msg string, args ...any) {
	slog.ErrorContext(ctx, msg, args...)
}

func WarnWithContext(ctx context.Context, msg string, args ...any) {
	slog.WarnContext(ctx, msg, args...)
}

func DebugWithContext(ctx context.Context, msg string, args ...any) {
	slog.DebugContext(ctx, msg, args...)
}
