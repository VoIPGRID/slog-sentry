// Package slogsentry provides a slog Handler wrapper for the Sentry-go library.
//
// Slog attributes are passed on to Sentry as context values.
// Some attribute keys are special. Attributes with a key prefixed with "tag_" are
// not stored in the context, but in the tags part of the Sentry event.
// The value of an attribute with the name "err" or "error" is included in the event
// message. Attributes with a name equal to a slog default key are ignored.
package slogsentry

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/getsentry/sentry-go"
)

const (
	shortErrKey = "err"
	longErrKey  = "error"

	// tagAttrPrefix defines the prefix slog attributes need to be detected as a value for
	// the Sentry tags map.
	tagAttrPrefix = "tag_"
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
		tags := map[string]string{}

		handleAttr := func(attr slog.Attr) {
			if strings.HasPrefix(attr.Key, tagAttrPrefix) {
				tags[attr.Key] = attr.Value.String()
			} else if !slices.Contains(slogDefaultKeys, attr.Key) {
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

			if len(tags) > 0 {
				scope.SetTags(tags)
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
	newHandler := NewSentryHandler(s.Handler.WithAttrs(attrs), s.levels)
	newHandler.storedAttrs = attrs

	return newHandler
}

// WithGroup returns a new SentryHandler whose group consists.
func (s *SentryHandler) WithGroup(name string) slog.Handler {
	return NewSentryHandler(s.Handler.WithGroup(name), s.levels)
}
