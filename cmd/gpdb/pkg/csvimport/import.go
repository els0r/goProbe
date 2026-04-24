package csvimport

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/els0r/goProbe/v4/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/v4/pkg/goDB"
	"github.com/els0r/goProbe/v4/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/els0r/goProbe/v4/pkg/types/hashmap"
)

var (
	errRowIPVersionMismatch = errors.New("ip version mismatch")
)

type keyFieldParser interface {
	ParseKey(element string, key *types.ExtendedKey) error
}

type keyParserItem struct {
	index    int
	priority int
	parser   keyFieldParser
}

type valParserItem struct {
	index  int
	parser goDB.StringValParser
}

type schemaDefinition struct {
	ifaceIndex int

	minFields int
	hasTime   bool

	keyParsers []keyParserItem
	valParsers []valParserItem
}

// Options controls CSV to goDB import behavior.
type Options struct {
	InputPath  string
	OutputPath string

	Schema    string
	Interface string

	MaxRows int

	EncoderType encoders.Type
	Permissions fs.FileMode
}

// Summary reports aggregate import results.
type Summary struct {
	RowsRead     int
	RowsImported int
	RowsSkipped  int

	Interfaces    int
	BlocksWritten int
}

// Import reads flow rows from CSV and writes them to a goDB directory.
func Import(ctx context.Context, opts Options) (Summary, error) {
	var summary Summary

	if opts.InputPath == "" {
		return summary, errors.New("input path must be non-empty")
	}
	if opts.OutputPath == "" {
		return summary, errors.New("output path must be non-empty")
	}
	if opts.MaxRows < 0 {
		return summary, fmt.Errorf("max rows must be >= 0, got %d", opts.MaxRows)
	}

	schema, reader, file, err := initCSVReader(opts)
	if err != nil {
		return summary, err
	}
	defer func() {
		_ = file.Close()
	}()

	if schema.ifaceIndex < 0 && strings.TrimSpace(opts.Interface) == "" {
		return summary, errors.New("schema does not contain `iface` field and no interface was provided via option")
	}

	permissions := opts.Permissions
	if permissions == 0 {
		permissions = goDB.DefaultPermissions
	}
	if opts.EncoderType > encoders.MaxEncoderType {
		return summary, fmt.Errorf("encoder type `%d` is out of range", opts.EncoderType)
	}

	pending := make(map[string]map[int64]*hashmap.AggFlowMap)
	writers := make(map[string]*goDB.DBWriter)

	baseV4 := types.NewEmptyV4Key().ExtendEmpty()
	baseV6 := types.NewEmptyV6Key().ExtendEmpty()

	currentTimestamp := int64(0)
	haveTimestamp := false

	for opts.MaxRows == 0 || summary.RowsRead < opts.MaxRows {
		select {
		case <-ctx.Done():
			return summary, ctx.Err()
		default:
		}

		row, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return summary, fmt.Errorf("failed to read CSV row %d from `%s`: %w", summary.RowsRead+1, opts.InputPath, readErr)
		}

		summary.RowsRead++

		if len(row) < schema.minFields {
			summary.RowsSkipped++
			continue
		}

		iface, key, counters, parseErr := parseRow(schema, row, strings.TrimSpace(opts.Interface), baseV4, baseV6)
		if parseErr != nil {
			summary.RowsSkipped++
			continue
		}

		timestamp, hasTime := key.AttrTime()
		if !hasTime {
			summary.RowsSkipped++
			continue
		}

		if haveTimestamp {
			if timestamp < currentTimestamp {
				return summary, fmt.Errorf("input must be ordered by non-decreasing timestamp (row=%d, previous=%d, current=%d)", summary.RowsRead, currentTimestamp, timestamp)
			}

			if timestamp > currentTimestamp {
				if err := flushBeforeTimestamp(ctx, pending, writers, opts.OutputPath, opts.EncoderType, permissions, timestamp, &summary); err != nil {
					return summary, err
				}
				currentTimestamp = timestamp
			}
		} else {
			haveTimestamp = true
			currentTimestamp = timestamp
		}

		if _, exists := pending[iface]; !exists {
			pending[iface] = make(map[int64]*hashmap.AggFlowMap)
		}
		if _, exists := pending[iface][timestamp]; !exists {
			pending[iface][timestamp] = hashmap.NewAggFlowMap()
		}

		if key.IsIPv4() {
			pending[iface][timestamp].PrimaryMap.Set(key.Key(), counters)
		} else {
			pending[iface][timestamp].SecondaryMap.Set(key.Key(), counters)
		}

		summary.RowsImported++
	}

	if err := flushAll(ctx, pending, writers, opts.OutputPath, opts.EncoderType, permissions, &summary); err != nil {
		return summary, err
	}

	summary.Interfaces = len(writers)

	return summary, nil
}

func initCSVReader(opts Options) (schemaDefinition, *csv.Reader, *os.File, error) {
	var schema schemaDefinition

	file, err := os.Open(opts.InputPath)
	if err != nil {
		return schema, nil, nil, fmt.Errorf("failed to open CSV input `%s`: %w", opts.InputPath, err)
	}

	reader := csv.NewReader(bufio.NewReader(file))
	reader.FieldsPerRecord = -1

	if strings.TrimSpace(opts.Schema) != "" {
		schema, err = parseSchema(opts.Schema)
		if err != nil {
			_ = file.Close()
			return schema, nil, nil, fmt.Errorf("failed to parse schema: %w", err)
		}
		return schema, reader, file, nil
	}

	header, err := reader.Read()
	if err != nil {
		_ = file.Close()
		if errors.Is(err, io.EOF) {
			return schema, nil, nil, fmt.Errorf("input CSV `%s` is empty", opts.InputPath)
		}
		return schema, nil, nil, fmt.Errorf("failed to read CSV header from `%s`: %w", opts.InputPath, err)
	}

	schema, err = parseSchema(strings.Join(header, ","))
	if err != nil {
		_ = file.Close()
		return schema, nil, nil, fmt.Errorf("failed to parse header schema: %w", err)
	}

	return schema, reader, file, nil
}

func flushBeforeTimestamp(
	ctx context.Context,
	pending map[string]map[int64]*hashmap.AggFlowMap,
	writers map[string]*goDB.DBWriter,
	outputPath string,
	encoderType encoders.Type,
	permissions fs.FileMode,
	cutoff int64,
	summary *Summary,
) error {
	for iface, byTimestamp := range pending {
		timestamps := make([]int64, 0, len(byTimestamp))
		for timestamp := range byTimestamp {
			if timestamp < cutoff {
				timestamps = append(timestamps, timestamp)
			}
		}

		if len(timestamps) == 0 {
			continue
		}

		sort.Slice(timestamps, func(i, j int) bool {
			return timestamps[i] < timestamps[j]
		})

		writer := getOrCreateWriter(writers, outputPath, iface, encoderType, permissions)

		for _, timestamp := range timestamps {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			flowMap := byTimestamp[timestamp]
			if err := writer.Write(flowMap, capturetypes.CaptureStats{}, timestamp); err != nil {
				return fmt.Errorf("failed to write block `%d` for interface `%s`: %w", timestamp, iface, err)
			}

			delete(byTimestamp, timestamp)
			summary.BlocksWritten++
		}

		if len(byTimestamp) == 0 {
			delete(pending, iface)
		}
	}

	return nil
}

func flushAll(
	ctx context.Context,
	pending map[string]map[int64]*hashmap.AggFlowMap,
	writers map[string]*goDB.DBWriter,
	outputPath string,
	encoderType encoders.Type,
	permissions fs.FileMode,
	summary *Summary,
) error {
	for iface, byTimestamp := range pending {
		timestamps := make([]int64, 0, len(byTimestamp))
		for timestamp := range byTimestamp {
			timestamps = append(timestamps, timestamp)
		}
		sort.Slice(timestamps, func(i, j int) bool {
			return timestamps[i] < timestamps[j]
		})

		writer := getOrCreateWriter(writers, outputPath, iface, encoderType, permissions)

		for _, timestamp := range timestamps {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			flowMap := byTimestamp[timestamp]
			if err := writer.Write(flowMap, capturetypes.CaptureStats{}, timestamp); err != nil {
				return fmt.Errorf("failed to write block `%d` for interface `%s`: %w", timestamp, iface, err)
			}
			summary.BlocksWritten++
		}
	}

	return nil
}

func getOrCreateWriter(
	writers map[string]*goDB.DBWriter,
	outputPath string,
	iface string,
	encoderType encoders.Type,
	permissions fs.FileMode,
) *goDB.DBWriter {
	if writer, exists := writers[iface]; exists {
		return writer
	}

	writer := goDB.NewDBWriter(outputPath, iface, encoderType).Permissions(permissions)
	writers[iface] = writer
	return writer
}

func parseRow(
	schema schemaDefinition,
	row []string,
	defaultIface string,
	baseV4 types.ExtendedKey,
	baseV6 types.ExtendedKey,
) (string, types.ExtendedKey, types.Counters, error) {
	iface := defaultIface
	if schema.ifaceIndex >= 0 {
		iface = strings.TrimSpace(row[schema.ifaceIndex])
	}
	if iface == "" {
		return "", nil, types.Counters{}, errors.New("empty interface")
	}

	key, err := parseKey(row, schema.keyParsers, baseV4, baseV6)
	if err != nil {
		return "", nil, types.Counters{}, err
	}

	var counters types.Counters
	for _, parser := range schema.valParsers {
		value := strings.TrimSpace(row[parser.index])
		if err := parser.parser.ParseVal(value, &counters); err != nil {
			return "", nil, types.Counters{}, fmt.Errorf("failed to parse value field %d: %w", parser.index, err)
		}
	}

	return iface, key, counters, nil
}

func parseKey(
	row []string,
	parsers []keyParserItem,
	baseV4 types.ExtendedKey,
	baseV6 types.ExtendedKey,
) (types.ExtendedKey, error) {
	key := baseV4.Clone()
	err := applyKeyParsers(row, parsers, &key)
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, errRowIPVersionMismatch) {
		return nil, err
	}

	key = baseV6.Clone()
	if err := applyKeyParsers(row, parsers, &key); err != nil {
		return nil, err
	}

	return key, nil
}

func applyKeyParsers(row []string, parsers []keyParserItem, key *types.ExtendedKey) error {
	for _, parser := range parsers {
		value := strings.TrimSpace(row[parser.index])
		if err := parser.parser.ParseKey(value, key); err != nil {
			if errors.Is(err, errRowIPVersionMismatch) {
				return err
			}
			return fmt.Errorf("failed to parse key field %d: %w", parser.index, err)
		}
	}

	return nil
}

func parseSchema(schema string) (schemaDefinition, error) {
	fields := strings.Split(schema, ",")

	def := schemaDefinition{
		ifaceIndex: -1,
		keyParsers: make([]keyParserItem, 0, len(fields)),
		valParsers: make([]valParserItem, 0, len(fields)),
	}

	parseableFields := 0

	for index, field := range fields {
		trimmedField := strings.ToLower(strings.TrimSpace(field))
		if trimmedField == "" {
			continue
		}

		switch trimmedField {
		case types.IfaceName:
			def.ifaceIndex = index
			parseableFields++
			def.minFields = max(def.minFields, index+1)
			continue
		case types.SIPName:
			def.keyParsers = append(def.keyParsers, keyParserItem{index: index, priority: 0, parser: sipStringParser{}})
			parseableFields++
			def.minFields = max(def.minFields, index+1)
			continue
		case types.DIPName:
			def.keyParsers = append(def.keyParsers, keyParserItem{index: index, priority: 0, parser: dipStringParser{}})
			parseableFields++
			def.minFields = max(def.minFields, index+1)
			continue
		}

		keyParser := goDB.NewStringKeyParser(trimmedField)
		if _, isNOP := keyParser.(*goDB.NOPStringParser); !isNOP {
			priority := 1
			def.keyParsers = append(def.keyParsers, keyParserItem{index: index, priority: priority, parser: keyParser})
			parseableFields++
			def.minFields = max(def.minFields, index+1)

			if trimmedField == types.TimeName {
				def.hasTime = true
			}

			continue
		}

		valParser := goDB.NewStringValParser(trimmedField)
		if _, isNOP := valParser.(*goDB.NOPStringParser); !isNOP {
			def.valParsers = append(def.valParsers, valParserItem{index: index, parser: valParser})
			parseableFields++
			def.minFields = max(def.minFields, index+1)
		}
	}

	if parseableFields == 0 {
		return def, errors.New("not a single field can be parsed in the provided schema")
	}
	if !def.hasTime {
		return def, errors.New("schema does not contain required `time` field")
	}

	sort.SliceStable(def.keyParsers, func(i, j int) bool {
		if def.keyParsers[i].priority == def.keyParsers[j].priority {
			return def.keyParsers[i].index < def.keyParsers[j].index
		}
		return def.keyParsers[i].priority < def.keyParsers[j].priority
	})

	sort.SliceStable(def.valParsers, func(i, j int) bool {
		return def.valParsers[i].index < def.valParsers[j].index
	})

	return def, nil
}

type sipStringParser struct{}

func (sipStringParser) ParseKey(element string, key *types.ExtendedKey) error {
	ipBytes, isIPv4, err := types.IPStringToBytes(element)
	if err != nil {
		return fmt.Errorf("could not parse `sip` attribute: %w", err)
	}
	if isIPv4 != key.IsIPv4() {
		return errRowIPVersionMismatch
	}

	key.PutSIP(ipBytes)
	return nil
}

type dipStringParser struct{}

func (dipStringParser) ParseKey(element string, key *types.ExtendedKey) error {
	ipBytes, isIPv4, err := types.IPStringToBytes(element)
	if err != nil {
		return fmt.Errorf("could not parse `dip` attribute: %w", err)
	}
	if isIPv4 != key.IsIPv4() {
		return errRowIPVersionMismatch
	}

	key.PutDIP(ipBytes)
	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ParsePermissions accepts numeric permission strings in decimal, octal or hex notation.
func ParsePermissions(raw string) (fs.FileMode, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}

	parsed, err := strconv.ParseUint(raw, 0, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid permissions `%s`: %w", raw, err)
	}

	return fs.FileMode(parsed), nil
}
