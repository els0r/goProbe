package convert

import (
	"testing"
	"time"

	"log/slog"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
)

type testLogValuer struct {
	a  string
	an int

	n nested
}

type nested struct {
	b string
}

func (t testLogValuer) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("a", t.a),
		slog.Int("an", t.an),
		slog.Any("n", t.n),
	)
}

func (n nested) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("b", n.b),
	)
}

func TestToKeyVal(t *testing.T) {
	var bigUint64 uint64 = 1<<64 - 42

	var tests = []struct {
		in       slog.Attr
		expected []attribute.KeyValue
	}{
		{slog.Bool("bool", true), []attribute.KeyValue{attribute.Bool("bool", true)}},
		{slog.Float64("float64", 1.0), []attribute.KeyValue{attribute.Float64("float64", 1.0)}},
		{slog.Int64("int64", 1), []attribute.KeyValue{attribute.Int64("int64", 1)}},
		{slog.Uint64("uint64", 42), []attribute.KeyValue{attribute.Int64("uint64", 42)}}, // the uint64 is too big
		{slog.Uint64("uint64", bigUint64), []attribute.KeyValue{}},                       // the uint64 is too big
		{slog.String("string", "string"), []attribute.KeyValue{attribute.String("string", "string")}},
		{slog.Time("time", time.Date(2020, 8, 12, 7, 47, 0, 0, time.UTC)),
			[]attribute.KeyValue{
				attribute.String("time", "2020-08-12T07:47:00Z"),
			},
		},
		{slog.Group("group",
			slog.String("string", "string"),
			slog.Int64("int", 1),
			slog.Group("subgroup",
				slog.Bool("bool", true),
			),
		),
			[]attribute.KeyValue{
				attribute.String("group.string", "string"),
				attribute.Int64("group.int", 1),
				attribute.Bool("group.subgroup.bool", true),
			},
		},
		// any object implementing the LogValuer interface
		{slog.Any("any", testLogValuer{a: "a", an: 1}),
			[]attribute.KeyValue{
				attribute.String("any.a", "a"),
				attribute.Int64("any.an", 1),
				attribute.String("any.n.b", ""),
			},
		},
		{slog.Any("anynested", testLogValuer{a: "a", an: 1, n: nested{b: "b"}}),
			[]attribute.KeyValue{
				attribute.String("anynested.a", "a"),
				attribute.Int64("anynested.an", 1),
				attribute.String("anynested.n.b", "b"),
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.in.Key, func(t *testing.T) {
			actual := ToKeyVals(test.in)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestToSlogAttr(t *testing.T) {
	var tests = []struct {
		in       attribute.KeyValue
		expected slog.Attr
	}{
		{attribute.Bool("bool", true), slog.Bool("bool", true)},
		{attribute.BoolSlice("bools", []bool{true, true}), slog.Any("bools", []bool{true, true})},
		{attribute.Float64("float64", 1.0), slog.Float64("float64", 1.0)},
		{attribute.Float64Slice("float64s", []float64{1.0, 1.2}), slog.Any("float64s", []float64{1.0, 1.2})},
		{attribute.Int64("int64", 1), slog.Int64("int64", 1)},
		{attribute.Int64Slice("int64s", []int64{1, 2}), slog.Any("int64s", []int64{1, 2})},
		{attribute.KeyValue{}, slog.Attr{}},
		{attribute.String("string", "string"), slog.String("string", "string")},
		{attribute.StringSlice("strings", []string{"a", "b"}), slog.Any("strings", []string{"a", "b"})},
	}

	for _, test := range tests {
		test := test
		t.Run(string(test.in.Key), func(t *testing.T) {
			actual := ToSlogAttr(test.in)
			assert.Equal(t, test.expected, actual)
		})
	}
}
