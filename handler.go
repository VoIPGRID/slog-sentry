package slogsentry

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/getsentry/sentry-go"
)

const (
	shortErrKey = "err"
	longErrKey  = "error"
)

var slogDefaultKeys = []string{slog.TimeKey, slog.LevelKey, slog.SourceKey, slog.MessageKey, shortErrKey, longErrKey}

// SlogEror contains both the slog msg and the actual error.
type SlogError struct {
	msg string
	err error
}

// Error appends both the msg and err from the SlogError.
func (e SlogError) Error() string {
	msg := e.msg
	if e.err != nil {
		if len(msg) > 0 {
			msg += ": "
		}
		msg += e.err.Error()
	}
	return msg
}

func (e SlogError) Unwrap() error {
	return e.err
}

// SentryHandler is a Handler that writes log records to the Sentry.
type SentryHandler struct {
	slog.Handler
	levels []slog.Level

	// storedAttrs allow to configure logging attributes which are always included in the context
	// of events reported to Sentry.
	storedAttrs []slog.Attr
}

// NewSentryHandler creates a SentryHandler that writes to w,
// using the given options.
func NewSentryHandler(
	handler slog.Handler,
	levels []slog.Level,
) *SentryHandler {
	return &SentryHandler{
		Handler: handler,
		levels:  levels,
	}
}

func NewSentryHandlerWithStoredAttrs(handler slog.Handler, levels []slog.Level, storedAttrs []slog.Attr) *SentryHandler {
	newHandler := NewSentryHandler(handler, levels)
	newHandler.storedAttrs = storedAttrs

	return newHandler
}

// Enabled reports whether the handler handles records at the given level.
func (s *SentryHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return s.Handler.Enabled(ctx, level)
}

// Handle intercepts and processes logger messages.
// In our case, send a message to the Sentry.
func (s *SentryHandler) Handle(ctx context.Context, record slog.Record) error {
	if slices.Contains(s.levels, record.Level) {
		hub := sentry.GetHubFromContext(ctx)
		if hub == nil {
			hub = sentry.CurrentHub()
		}
		if hub == nil {
			return fmt.Errorf("sentry: hub is nil")
		}

		var err error
		slogContext := map[string]any{}

		handleAttr := func(attr slog.Attr) {
			if !slices.Contains(slogDefaultKeys, attr.Key) {
				slogContext[attr.Key] = attr.Value.String()
			} else if attr.Key == shortErrKey || attr.Key == longErrKey {
				var ok bool
				err, ok = attr.Value.Any().(error)
				if !ok {
					slogContext[attr.Key] = attr.Value.String()
				}
			}
		}

		for _, attr := range s.storedAttrs {
			handleAttr(attr)
		}

		record.Attrs(func(attr slog.Attr) bool {
			handleAttr(attr)
			return true
		})

		hub.WithScope(func(scope *sentry.Scope) {
			if len(slogContext) > 0 {
				scope.SetContext("slog", slogContext)
			}

			switch record.Level {
			case slog.LevelError:
				sentry.CaptureException(SlogError{msg: record.Message, err: err})
			case slog.LevelDebug, slog.LevelInfo, slog.LevelWarn:
				sentry.CaptureMessage(record.Message)
			}
		})
	}

	return s.Handler.Handle(ctx, record)
}

// WithAttrs returns a new SentryHandler with the given attributes stored.
func (s *SentryHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return NewSentryHandlerWithStoredAttrs(
		s.Handler.WithAttrs(attrs),
		s.levels,
		attrs)
}

// WithGroup returns a new SentryHandler whose group consists.
func (s *SentryHandler) WithGroup(name string) slog.Handler {
	return NewSentryHandler(s.Handler.WithGroup(name), s.levels)
}
