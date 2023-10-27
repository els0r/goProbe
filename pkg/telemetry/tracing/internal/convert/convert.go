package convert

import (
	"time"

	"log/slog"

	"go.opentelemetry.io/otel/attribute"
)

const hierarchySep = "."

// ToKeyVals converts a slog.Attr to a list of attribute.KeyValue objects. Nested values
// such as slog.Group and slog.LogValuer are flattened and their hierarchy represented by
// a delimiting "."
func ToKeyVals(attr slog.Attr) (kvs []attribute.KeyValue) {
	key := attr.Key
	val := attr.Value

	kvs = []attribute.KeyValue{}

	switch val.Kind() {
	case slog.KindAny:
		// TODO: implement/figure out
		return kvs
	case slog.KindBool:
		return []attribute.KeyValue{attribute.Bool(key, val.Bool())}
	case slog.KindDuration:
	case slog.KindFloat64:
		return []attribute.KeyValue{attribute.Float64(key, val.Float64())}
	case slog.KindInt64:
		return []attribute.KeyValue{attribute.Int64(key, val.Int64())}
	case slog.KindString:
		return []attribute.KeyValue{attribute.String(key, val.String())}
	case slog.KindTime:
		// we "cast" to a time string with appropriate precision
		return []attribute.KeyValue{attribute.String(key, val.Time().Format(time.RFC3339Nano))}
	case slog.KindUint64:
		// make sure the int64 doesn't overflow. Don't return
		// an attribute if the uint64 is bigger than the allowed
		// maximum
		var int64max uint64 = 1<<63 - 1
		if val.Uint64() > int64max {
			return kvs
		}
		return []attribute.KeyValue{attribute.Int64(key, int64(val.Uint64()))}
	case slog.KindGroup:
		// recurse into the group
		for _, a := range val.Group() {
			a.Key = key + hierarchySep + a.Key
			kvs = append(kvs, ToKeyVals(a)...)
		}
		return kvs
	case slog.KindLogValuer:
		// recurse into the LogValuer
		kvs = append(kvs, ToKeyVals(slog.Any(key, val.LogValuer().LogValue()))...)
	}
	return kvs
}

// ToSlogAttr converts an attribute.KeyValue to a slog.Attr
func ToSlogAttr(kv attribute.KeyValue) (attr slog.Attr) {
	key := string(kv.Key)
	val := kv.Value

	switch val.Type() {
	case attribute.BOOL:
		return slog.Bool(key, val.AsBool())
	case attribute.BOOLSLICE:
		return slog.Any(key, val.AsBoolSlice())
	case attribute.FLOAT64:
		return slog.Float64(key, val.AsFloat64())
	case attribute.FLOAT64SLICE:
		return slog.Any(key, val.AsFloat64Slice())
	case attribute.INT64:
		return slog.Int64(key, val.AsInt64())
	case attribute.INT64SLICE:
		return slog.Any(key, val.AsInt64Slice())
	case attribute.INVALID:
		// empty Attr objects aren't logged, so it's safe to just return
		return attr
	case attribute.STRING:
		return slog.String(key, val.AsString())
	case attribute.STRINGSLICE:
		return slog.Any(key, val.AsStringSlice())
	}
	return attr
}
