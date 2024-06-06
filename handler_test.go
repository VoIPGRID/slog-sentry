package slogsentry

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

func TestSlogErrorErrorMethod(t *testing.T) {
	tests := []struct {
		input        SlogError
		expectOutput string
	}{
		{SlogError{msg: "the message", err: errors.New("the error")}, "the message: the error"},
		{SlogError{err: errors.New("the error")}, "the error"},
		{SlogError{msg: "the message"}, "the message"},
	}

	for i, test := range tests {
		output := test.input.Error()
		if output != test.expectOutput {
			t.Errorf("test %d: expect: %q, got: %q", i, test.expectOutput, output)
		}
	}
}

func TestHandleHandlesNilErrorAttr(t *testing.T) {
	record := slog.NewRecord(time.Now(), slog.LevelError, "the message", uintptr(0))
	record.AddAttrs(slog.Any("some_attr", "yes"), slog.Any("error", nil))

	handler := NewSentryHandler(slog.Default().Handler(), []slog.Level{slog.LevelError})
	err := handler.Handle(context.Background(), record)
	if err != nil {
		t.Errorf("error from Handle: %s", err)
	}
}
