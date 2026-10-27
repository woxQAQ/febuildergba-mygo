// Package fault describes failures independently of UI text and dependencies.
package fault

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"log/slog"
	"os"
	"strings"
)

// Kind groups failures by the response available to the caller. A conflict
// requires a fresh read or explicit resolution; it is not safe to retry a
// write.
type Kind string

const (
	Unknown      Kind = "unknown"
	InvalidInput Kind = "invalid_input"
	InvalidData  Kind = "invalid_data"
	Unsupported  Kind = "unsupported"
	Conflict     Kind = "conflict"
	NotFound     Kind = "not_found"
	Permission   Kind = "permission"
	Capacity     Kind = "capacity"
	IO           Kind = "io"
	Canceled     Kind = "canceled"
)

// Code is a comparable failure definition owned by the package that detects it.
// ID is stable and independent of Error's diagnostic formatting or UI catalogs.
type Code struct {
	ID   string
	Kind Kind
}

func (c Code) Error() string { return c.ID }

// Context contains facts about an operation. Optional numeric groups preserve
// zero addresses, IDs and revisions without treating zero as missing.
type Context struct {
	Path       string
	Table      string
	Field      string
	Resource   string
	RecordID   *int
	ResourceID *int
	Range      *Range
	Value      *int64
	Bounds     *Bounds
	Revision   *Revision
}

type (
	Range    struct{ Offset, Length, Size int }
	Bounds   struct{ Min, Max int64 }
	Revision struct{ Expected, Actual uint64 }
)

// Error adds one operation frame. Code describes the caller-facing failure;
// Cause retains the underlying error, including errors.Join trees.
type Error struct {
	Code    Code
	Op      string
	Context Context
	Cause   error
}

// New translates a failure at a semantic boundary without losing its cause.
func New(code Code, op string, ctx Context, cause error) *Error {
	return &Error{Code: code, Op: op, Context: ctx, Cause: cause}
}

// Wrap adds context without changing the meaning of a failure. Success stays
// nil, so callers may use it on a direct return from a dependency.
func Wrap(err error, op string, ctx Context) error {
	if err == nil {
		return nil
	}
	return New(CodeOf(err), op, ctx, err)
}

func (e *Error) Unwrap() error { return e.Cause }

func (e *Error) Is(target error) bool {
	code, ok := target.(Code)
	return ok && e.Code == code
}

func (e *Error) Error() string {
	var out strings.Builder
	out.WriteString(e.Op)
	out.WriteString(": ")
	out.WriteString(e.Code.ID)
	for _, attr := range e.Context.attrs() {
		fmt.Fprintf(&out, " %s=%v", attr.Key, attr.Value)
	}
	if e.Cause != nil {
		if code, ok := e.Cause.(Code); !ok || code != e.Code {
			fmt.Fprintf(&out, ": %v", e.Cause)
		}
	}
	return out.String()
}

// CodeOf selects the outermost semantic code. Joined failures retain their
// order: the first non-nil cause is primary, including an unknown failure.
// Cleanup failures therefore cannot replace the reason an operation failed.
func CodeOf(err error) Code {
	if err == nil {
		return Code{}
	}
	switch e := err.(type) {
	case *Error:
		return e.Code
	case Code:
		return e
	case interface{ Unwrap() []error }:
		for _, cause := range e.Unwrap() {
			if cause != nil {
				return CodeOf(cause)
			}
		}
	case interface{ Unwrap() error }:
		// Classify OS failures before unwrapping so an unknown errno still
		// retains its filesystem operation category.
		switch e.(type) {
		case *fs.PathError, *os.LinkError:
		default:
			if cause := e.Unwrap(); cause != nil {
				return CodeOf(cause)
			}
		}
	}
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Code{"fault.not_found", NotFound}
	case errors.Is(err, fs.ErrPermission):
		return Code{"fault.permission", Permission}
	case errors.Is(err, fs.ErrExist):
		return Code{"fault.exists", Conflict}
	case errors.Is(err, context.Canceled):
		return Code{"fault.canceled", Canceled}
	default:
		switch err.(type) {
		case *fs.PathError, *os.LinkError:
			return Code{"fault.io", IO}
		}
		return Code{"fault.unknown", Unknown}
	}
}

// LogValue keeps the operation tree machine-readable, including all secondary
// failures. Error() remains available as an unlocalized diagnostic for humans.
func (e *Error) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("code", e.Code.ID),
		slog.String("kind", string(e.Code.Kind)),
		slog.String("operation", e.Op),
	}
	attrs = append(attrs, e.Context.attrs()...)
	if e.Cause != nil {
		attrs = append(attrs, slog.Any("cause", LogValue(e.Cause)))
	}
	return slog.GroupValue(attrs...)
}

func LogValue(err error) slog.Value {
	if err == nil {
		return slog.AnyValue(nil)
	}
	if e, ok := err.(*Error); ok {
		return e.LogValue()
	}
	code := CodeOf(err)
	attrs := []slog.Attr{
		slog.String("code", code.ID),
		slog.String("kind", string(code.Kind)),
		slog.String("detail", err.Error()),
	}
	switch e := err.(type) {
	case *fs.PathError:
		attrs = append(
			attrs,
			slog.String("operation", e.Op),
			slog.String("path", e.Path),
		)
	case *os.LinkError:
		attrs = append(
			attrs,
			slog.String("operation", e.Op),
			slog.String("source", e.Old),
			slog.String("destination", e.New),
		)
	}
	switch e := err.(type) {
	case interface{ Unwrap() []error }:
		var causes []slog.Attr
		for i, cause := range e.Unwrap() {
			if cause != nil {
				causes = append(
					causes,
					slog.Any(fmt.Sprint(i), LogValue(cause)),
				)
			}
		}
		attrs = append(attrs, slog.Any("causes", slog.GroupValue(causes...)))
	case interface{ Unwrap() error }:
		if cause := e.Unwrap(); cause != nil {
			attrs = append(attrs, slog.Any("cause", LogValue(cause)))
		}
	}
	return slog.GroupValue(attrs...)
}

// Frames visits operation contexts in outermost-first, depth-first order.
// Joined secondary failures remain available to diagnostic consumers.
func Frames(err error) iter.Seq[*Error] {
	return func(yield func(*Error) bool) {
		var walk func(error) bool
		walk = func(err error) bool {
			if e, ok := err.(*Error); ok && !yield(e) {
				return false
			}
			switch e := err.(type) {
			case interface{ Unwrap() []error }:
				for _, cause := range e.Unwrap() {
					if !walk(cause) {
						return false
					}
				}
			case interface{ Unwrap() error }:
				return walk(e.Unwrap())
			}
			return true
		}
		walk(err)
	}
}

func (c Context) attrs() []slog.Attr {
	var attrs []slog.Attr
	for _, attr := range []slog.Attr{
		slog.String("path", c.Path), slog.String("table", c.Table),
		slog.String("field", c.Field), slog.String("resource", c.Resource),
	} {
		if attr.Value.String() != "" {
			attrs = append(attrs, attr)
		}
	}
	if c.RecordID != nil {
		attrs = append(attrs, slog.Int("record_id", *c.RecordID))
	}
	if c.ResourceID != nil {
		attrs = append(attrs, slog.Int("resource_id", *c.ResourceID))
	}
	if c.Range != nil {
		attrs = append(
			attrs,
			slog.Int("offset", c.Range.Offset),
			slog.Int("length", c.Range.Length),
			slog.Int("size", c.Range.Size),
		)
	}
	if c.Value != nil {
		attrs = append(attrs, slog.Int64("value", *c.Value))
	}
	if c.Bounds != nil {
		attrs = append(
			attrs,
			slog.Int64("min", c.Bounds.Min),
			slog.Int64("max", c.Bounds.Max),
		)
	}
	if c.Revision != nil {
		attrs = append(
			attrs,
			slog.Uint64("expected_revision", c.Revision.Expected),
			slog.Uint64("actual_revision", c.Revision.Actual),
		)
	}
	return attrs
}
