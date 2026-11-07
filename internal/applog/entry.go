package applog

import (
	"bytes"
	"context"
	"log/slog"
)

// JSON uses the same structured representation as the persistent log. The
// sequence is an in-memory identity and does not alter the recorded attributes.
func (entry Entry) JSON() []byte {
	var output bytes.Buffer
	record := slog.NewRecord(entry.Time, entry.Level, entry.Message, 0)
	record.AddAttrs(entry.Attrs...)
	// A bytes.Buffer cannot fail writes.
	_ = slog.NewJSONHandler(&output, nil).Handle(context.Background(), record)
	return output.Bytes()
}
