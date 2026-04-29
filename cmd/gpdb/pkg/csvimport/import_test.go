package csvimport

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/els0r/goProbe/v4/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/v4/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/fako1024/gotools/bitpack"
	"github.com/stretchr/testify/require"
)

func TestImportWithHeaderSchemaAndIfaceColumn(t *testing.T) {
	t.Parallel()

	inputPath := filepath.Join(t.TempDir(), "input.csv")
	outPath := t.TempDir()

	content := "time,iface,sip,dip,dport,proto,packets received,packets sent,data vol. received,data vol. sent\n" +
		"1711929900,eth0,10.0.0.1,10.0.0.2,443,TCP,3,2,300,200\n" +
		"1711929900,eth0,10.0.0.3,10.0.0.4,80,TCP,4,1,400,100\n" +
		"1711930200,eth0,10.0.0.5,10.0.0.6,53,UDP,1,1,50,50\n"
	require.NoError(t, os.WriteFile(inputPath, []byte(content), 0o600))

	summary, err := Import(context.Background(), Options{
		InputPath:   inputPath,
		OutputPath:  outPath,
		EncoderType: encoders.EncoderTypeLZ4,
	})
	require.NoError(t, err)
	require.Equal(t, 3, summary.RowsRead)
	require.Equal(t, 3, summary.RowsImported)
	require.Equal(t, 0, summary.RowsSkipped)
	require.Equal(t, 1, summary.Interfaces)
	require.Equal(t, 2, summary.BlocksWritten)

	desc := mustSingleDayDescriptor(t, filepath.Join(outPath, "eth0"))
	countersByTimestamp, err := readDayCounters(filepath.Join(outPath, "eth0"), desc)
	require.NoError(t, err)
	require.Len(t, countersByTimestamp, 2)

	require.EqualValues(t, 700, countersByTimestamp[1711929900].BytesRcvd)
	require.EqualValues(t, 300, countersByTimestamp[1711929900].BytesSent)
	require.EqualValues(t, 7, countersByTimestamp[1711929900].PacketsRcvd)
	require.EqualValues(t, 3, countersByTimestamp[1711929900].PacketsSent)

	require.EqualValues(t, 50, countersByTimestamp[1711930200].BytesRcvd)
	require.EqualValues(t, 50, countersByTimestamp[1711930200].BytesSent)
	require.EqualValues(t, 1, countersByTimestamp[1711930200].PacketsRcvd)
	require.EqualValues(t, 1, countersByTimestamp[1711930200].PacketsSent)
}

func TestImportWithProvidedSchemaAndIfaceOverride(t *testing.T) {
	t.Parallel()

	inputPath := filepath.Join(t.TempDir(), "input.csv")
	outPath := t.TempDir()

	content := "1711929900,10.0.0.1,10.0.0.2,443,TCP,3,2,300,200\n" +
		"1711930200,10.0.0.3,10.0.0.4,80,TCP,4,1,400,100\n"
	require.NoError(t, os.WriteFile(inputPath, []byte(content), 0o600))

	summary, err := Import(context.Background(), Options{
		InputPath:   inputPath,
		OutputPath:  outPath,
		Schema:      "time,sip,dip,dport,proto,packets received,packets sent,data vol. received,data vol. sent",
		Interface:   "eth1",
		EncoderType: encoders.EncoderTypeLZ4,
	})
	require.NoError(t, err)
	require.Equal(t, 2, summary.RowsImported)

	days, err := listInterfaceDays(filepath.Join(outPath, "eth1"))
	require.NoError(t, err)
	require.Len(t, days, 1)
}

func TestImportWithMaxRows(t *testing.T) {
	t.Parallel()

	inputPath := filepath.Join(t.TempDir(), "input.csv")
	outPath := t.TempDir()

	content := "time,iface,sip,dip,dport,proto,packets received,packets sent,data vol. received,data vol. sent\n" +
		"1711929900,eth0,10.0.0.1,10.0.0.2,443,TCP,3,2,300,200\n" +
		"1711930200,eth0,10.0.0.3,10.0.0.4,80,TCP,4,1,400,100\n"
	require.NoError(t, os.WriteFile(inputPath, []byte(content), 0o600))

	summary, err := Import(context.Background(), Options{
		InputPath:   inputPath,
		OutputPath:  outPath,
		MaxRows:     1,
		EncoderType: encoders.EncoderTypeLZ4,
	})
	require.NoError(t, err)
	require.Equal(t, 1, summary.RowsRead)
	require.Equal(t, 1, summary.RowsImported)
	require.Equal(t, 1, summary.BlocksWritten)
}

func TestImportSkipsMalformedRows(t *testing.T) {
	t.Parallel()

	inputPath := filepath.Join(t.TempDir(), "input.csv")
	outPath := t.TempDir()

	content := "time,iface,sip,dip,dport,proto,packets received,packets sent,data vol. received,data vol. sent\n" +
		"1711929900,eth0,10.0.0.1,10.0.0.2,443,TCP,3,2,300,200\n" +
		"1711930200,eth0,INVALID_IP,10.0.0.4,80,TCP,4,1,400,100\n"
	require.NoError(t, os.WriteFile(inputPath, []byte(content), 0o600))

	summary, err := Import(context.Background(), Options{
		InputPath:   inputPath,
		OutputPath:  outPath,
		EncoderType: encoders.EncoderTypeLZ4,
	})
	require.NoError(t, err)
	require.Equal(t, 2, summary.RowsRead)
	require.Equal(t, 1, summary.RowsImported)
	require.Equal(t, 1, summary.RowsSkipped)
}

func TestImportRequiresInterfaceWhenSchemaHasNoIface(t *testing.T) {
	t.Parallel()

	inputPath := filepath.Join(t.TempDir(), "input.csv")
	outPath := t.TempDir()

	content := "1711929900,10.0.0.1,10.0.0.2,443,TCP,3,2,300,200\n"
	require.NoError(t, os.WriteFile(inputPath, []byte(content), 0o600))

	_, err := Import(context.Background(), Options{
		InputPath:   inputPath,
		OutputPath:  outPath,
		Schema:      "time,sip,dip,dport,proto,packets received,packets sent,data vol. received,data vol. sent",
		EncoderType: encoders.EncoderTypeLZ4,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no interface was provided")
}

func TestImportContextCanceled(t *testing.T) {
	t.Parallel()

	inputPath := filepath.Join(t.TempDir(), "input.csv")
	outPath := t.TempDir()

	content := "time,iface,sip,dip,dport,proto,packets received,packets sent,data vol. received,data vol. sent\n" +
		"1711929900,eth0,10.0.0.1,10.0.0.2,443,TCP,3,2,300,200\n"
	require.NoError(t, os.WriteFile(inputPath, []byte(content), 0o600))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Import(ctx, Options{
		InputPath:   inputPath,
		OutputPath:  outPath,
		EncoderType: encoders.EncoderTypeLZ4,
	})
	require.ErrorIs(t, err, context.Canceled)
}

func TestParsePermissions(t *testing.T) {
	t.Parallel()

	perm, err := ParsePermissions("0644")
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), perm)

	perm, err = ParsePermissions("420")
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), perm)

	_, err = ParsePermissions("not-a-number")
	require.Error(t, err)
}

func TestImportRejectsInvalidEncoderType(t *testing.T) {
	t.Parallel()

	inputPath := filepath.Join(t.TempDir(), "input.csv")
	outPath := t.TempDir()

	content := "time,iface,sip,dip,dport,proto,packets received,packets sent,data vol. received,data vol. sent\n" +
		"1711929900,eth0,10.0.0.1,10.0.0.2,443,TCP,3,2,300,200\n"
	require.NoError(t, os.WriteFile(inputPath, []byte(content), 0o600))

	_, err := Import(context.Background(), Options{
		InputPath:   inputPath,
		OutputPath:  outPath,
		EncoderType: encoders.MaxEncoderType + 1,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of range")
}

func TestImportRequiresNonDecreasingTimestamps(t *testing.T) {
	t.Parallel()

	inputPath := filepath.Join(t.TempDir(), "input.csv")
	outPath := t.TempDir()

	content := "time,iface,sip,dip,dport,proto,packets received,packets sent,data vol. received,data vol. sent\n" +
		"1711930200,eth0,10.0.0.3,10.0.0.4,80,TCP,4,1,400,100\n" +
		"1711929900,eth0,10.0.0.1,10.0.0.2,443,TCP,3,2,300,200\n"
	require.NoError(t, os.WriteFile(inputPath, []byte(content), 0o600))

	summary, err := Import(context.Background(), Options{
		InputPath:   inputPath,
		OutputPath:  outPath,
		EncoderType: encoders.EncoderTypeLZ4,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "input must be ordered")
	require.Equal(t, 1, summary.RowsImported)
}

type dayDescriptor struct {
	Timestamp int64
	Suffix    string
	DirName   string
	Path      string
}

func listInterfaceDays(ifacePath string) (map[int64]dayDescriptor, error) {
	result := make(map[int64]dayDescriptor)

	entries, err := os.ReadDir(ifacePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return result, nil
		}
		return nil, err
	}

	for _, yearEntry := range entries {
		if !yearEntry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(yearEntry.Name()); err != nil {
			continue
		}

		yearPath := filepath.Join(ifacePath, yearEntry.Name())
		monthEntries, err := os.ReadDir(yearPath)
		if err != nil {
			return nil, err
		}

		for _, monthEntry := range monthEntries {
			if !monthEntry.IsDir() {
				continue
			}
			if _, err := strconv.Atoi(monthEntry.Name()); err != nil {
				continue
			}

			monthPath := filepath.Join(yearPath, monthEntry.Name())
			dayEntries, err := os.ReadDir(monthPath)
			if err != nil {
				return nil, err
			}

			for _, dayEntry := range dayEntries {
				if !dayEntry.IsDir() {
					continue
				}

				dayTimestamp, suffix, err := gpfile.ExtractTimestampMetadataSuffix(dayEntry.Name())
				if err != nil {
					return nil, err
				}

				result[dayTimestamp] = dayDescriptor{
					Timestamp: dayTimestamp,
					Suffix:    suffix,
					DirName:   dayEntry.Name(),
					Path:      filepath.Join(monthPath, dayEntry.Name()),
				}
			}
		}
	}

	return result, nil
}

func mustSingleDayDescriptor(t *testing.T, ifacePath string) dayDescriptor {
	t.Helper()

	days, err := listInterfaceDays(ifacePath)
	require.NoError(t, err)
	require.Len(t, days, 1)

	for _, day := range days {
		return day
	}

	return dayDescriptor{}
}

func readDayCounters(ifacePath string, day dayDescriptor) (map[int64]types.Counters, error) {
	reader := gpfile.NewDirReader(ifacePath, day.Timestamp, day.Suffix)
	if err := reader.Open(); err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close()
	}()

	countersByTimestamp := make(map[int64]types.Counters, reader.NBlocks())
	for blockIndex, block := range reader.BlockMetadata[0].Blocks() {
		var data [types.ColIdxCount][]byte
		for i := range int(types.ColIdxCount) {
			columnIndex := types.ColumnIndex(i)
			blockData, err := reader.ReadBlockAtIndex(columnIndex, blockIndex)
			if err != nil {
				return nil, err
			}
			data[i] = append([]byte(nil), blockData...)
		}

		counters, err := decodeCounters(data)
		if err != nil {
			return nil, err
		}

		countersByTimestamp[block.Timestamp] = counters
	}

	return countersByTimestamp, nil
}

func decodeCounters(data [types.ColIdxCount][]byte) (types.Counters, error) {
	bytesRcvdValues := bitpack.UnpackInto(data[types.BytesRcvdColIdx], nil)
	bytesSentValues := bitpack.UnpackInto(data[types.BytesSentColIdx], nil)
	pktsRcvdValues := bitpack.UnpackInto(data[types.PacketsRcvdColIdx], nil)
	pktsSentValues := bitpack.UnpackInto(data[types.PacketsSentColIdx], nil)

	n := len(bytesRcvdValues)
	if len(bytesSentValues) != n || len(pktsRcvdValues) != n || len(pktsSentValues) != n {
		return types.Counters{}, errors.New("counter columns differ in length")
	}

	var counters types.Counters
	for i := 0; i < n; i++ {
		counters.BytesRcvd += bytesRcvdValues[i]
		counters.BytesSent += bytesSentValues[i]
		counters.PacketsRcvd += pktsRcvdValues[i]
		counters.PacketsSent += pktsSentValues[i]
	}

	return counters, nil
}
