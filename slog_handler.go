package nikologs

import (
	"context"
	"log/slog"
)

// SlogHandlerOptions configures the slog handler.
type SlogHandlerOptions struct {
	// Level is the minimum slog level that will be forwarded to nikologs.
	// Defaults to slog.LevelInfo if nil/zero.
	Level slog.Leveler
}

// slogHandler implements slog.Handler, forwarding records to a nikologs Client.
type slogHandler struct {
	client *Client
	level  slog.Leveler
	group  string // dot-separated group prefix
	attrs  []slog.Attr
}

// NewSlogHandler returns an slog.Handler that forwards log records to the given Client.
func NewSlogHandler(client *Client, opts *SlogHandlerOptions) slog.Handler {
	h := &slogHandler{
		client: client,
		level:  slog.LevelInfo,
	}
	if opts != nil && opts.Level != nil {
		h.level = opts.Level
	}
	return h
}

// Enabled reports whether the handler is configured to handle records at the given level.
func (h *slogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

// Handle converts the slog.Record to a nikologs entry and buffers it.
func (h *slogHandler) Handle(_ context.Context, r slog.Record) error {
	fields := make(Fields)

	// Add pre-set attrs
	for _, a := range h.attrs {
		h.addAttr(fields, h.group, a)
	}

	// Add record attrs
	r.Attrs(func(a slog.Attr) bool {
		h.addAttr(fields, h.group, a)
		return true
	})

	var meta Fields
	if len(fields) > 0 {
		meta = fields
	}

	h.client.Log(h.mapLevel(r.Level), r.Message, meta)
	return nil
}

// WithAttrs returns a new handler with the given attributes pre-set.
func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &slogHandler{
		client: h.client,
		level:  h.level,
		group:  h.group,
		attrs:  newAttrs,
	}
}

// WithGroup returns a new handler with the given group prefix.
func (h *slogHandler) WithGroup(name string) slog.Handler {
	newGroup := name
	if h.group != "" {
		newGroup = h.group + "." + name
	}
	return &slogHandler{
		client: h.client,
		level:  h.level,
		group:  newGroup,
		attrs:  h.attrs,
	}
}

// mapLevel converts a slog.Level to a nikologs Level.
func (h *slogHandler) mapLevel(l slog.Level) Level {
	switch {
	case l < slog.LevelDebug:
		return LevelTrace
	case l < slog.LevelInfo:
		return LevelDebug
	case l < slog.LevelWarn:
		return LevelInfo
	case l < slog.LevelError:
		return LevelWarn
	default:
		return LevelError
	}
}

// addAttr adds a single slog.Attr to the fields map with the given group prefix.
func (h *slogHandler) addAttr(fields Fields, prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	key := a.Key
	if prefix != "" {
		key = prefix + "." + key
	}

	if a.Value.Kind() == slog.KindGroup {
		for _, ga := range a.Value.Group() {
			h.addAttr(fields, key, ga)
		}
		return
	}

	switch a.Value.Kind() {
	case slog.KindInt64:
		fields[key] = int(a.Value.Int64())
	case slog.KindUint64:
		fields[key] = uint(a.Value.Uint64())
	default:
		fields[key] = a.Value.Any()
	}
}
