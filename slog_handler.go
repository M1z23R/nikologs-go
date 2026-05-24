package nikologs

import (
	"context"
	"log/slog"
)

// defaultTagsKey is the attr key that routes into a log entry's tags
// instead of its metadata.
const defaultTagsKey = "tags"

// SlogHandlerOptions configures the slog handler.
type SlogHandlerOptions struct {
	// Level is the minimum slog level that will be forwarded to nikologs.
	// Defaults to slog.LevelInfo if nil/zero.
	Level slog.Leveler
	// TagsKey is the top-level attr key whose value becomes the entry's
	// tags rather than metadata. Defaults to "tags". The value may be a
	// []string, []any of strings, or a single string. Only recognized at
	// the top level — a key under a WithGroup prefix stays in metadata.
	TagsKey string
}

// slogHandler implements slog.Handler, forwarding records to a nikologs Client.
type slogHandler struct {
	client  *Client
	level   slog.Leveler
	group   string // dot-separated group prefix
	attrs   []slog.Attr
	tagsKey string
}

// NewSlogHandler returns an slog.Handler that forwards log records to the given Client.
func NewSlogHandler(client *Client, opts *SlogHandlerOptions) slog.Handler {
	h := &slogHandler{
		client:  client,
		level:   slog.LevelInfo,
		tagsKey: defaultTagsKey,
	}
	if opts != nil {
		if opts.Level != nil {
			h.level = opts.Level
		}
		if opts.TagsKey != "" {
			h.tagsKey = opts.TagsKey
		}
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
	var tags []string

	collect := func(a slog.Attr) {
		if h.group == "" && a.Key == h.tagsKey {
			if t := attrToTags(a.Value); t != nil {
				tags = append(tags, t...)
				return
			}
		}
		h.addAttr(fields, h.group, a)
	}

	// Add pre-set attrs
	for _, a := range h.attrs {
		collect(a)
	}

	// Add record attrs
	r.Attrs(func(a slog.Attr) bool {
		collect(a)
		return true
	})

	var meta Fields
	if len(fields) > 0 {
		meta = fields
	}

	var opts []LogOption
	if len(tags) > 0 {
		opts = append(opts, WithTags(tags...))
	}

	h.client.Log(h.mapLevel(r.Level), r.Message, meta, opts...)
	return nil
}

// WithAttrs returns a new handler with the given attributes pre-set.
func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &slogHandler{
		client:  h.client,
		level:   h.level,
		group:   h.group,
		attrs:   newAttrs,
		tagsKey: h.tagsKey,
	}
}

// WithGroup returns a new handler with the given group prefix.
func (h *slogHandler) WithGroup(name string) slog.Handler {
	newGroup := name
	if h.group != "" {
		newGroup = h.group + "." + name
	}
	return &slogHandler{
		client:  h.client,
		level:   h.level,
		group:   newGroup,
		attrs:   h.attrs,
		tagsKey: h.tagsKey,
	}
}

// attrToTags converts a reserved tags attr value to a slice of tag strings.
// It accepts a []string, a []any of strings, or a single string. Any other
// kind returns nil, signaling the attr should be treated as normal metadata.
func attrToTags(v slog.Value) []string {
	v = v.Resolve()
	switch v.Kind() {
	case slog.KindString:
		if s := v.String(); s != "" {
			return []string{s}
		}
		return nil
	case slog.KindAny:
		switch t := v.Any().(type) {
		case []string:
			return t
		case []any:
			out := make([]string, 0, len(t))
			for _, e := range t {
				if s, ok := e.(string); ok {
					out = append(out, s)
				}
			}
			return out
		}
	}
	return nil
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
