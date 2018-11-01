/////////////////////////////////////////////////////////////////////////////////
//
// TablePrinter_test.go
//
// Written by Lorenz Breidenbach lob@open.ch, February 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package main

import (
    "OSAG/goDB"
    "bytes"
    "encoding/json"
    "os"
    "reflect"
    "regexp"
    "strings"
    "testing"
    "time"
)

var columnsTests = []struct {
    queryType string
    direction Direction
    output    []OutputColumn
}{
    {
        "sip,dip",
        DIRECTION_IN,
        []OutputColumn{OUTCOL_SIP, OUTCOL_DIP,
            OUTCOL_INPKTS, OUTCOL_INPKTSPERCENT,
            OUTCOL_INBYTES, OUTCOL_INBYTESPERCENT,
        },
    },
    {
        "l7proto,dport,proto",
        DIRECTION_OUT,
        []OutputColumn{OUTCOL_L7PROTOID, OUTCOL_L7PROTOCATEGORY,
            OUTCOL_DPORT, OUTCOL_PROTO,
            OUTCOL_OUTPKTS, OUTCOL_OUTPKTSPERCENT,
            OUTCOL_OUTBYTES, OUTCOL_OUTBYTESPERCENT,
        },
    },
    {
        "sip,proto,time",
        DIRECTION_BOTH,
        []OutputColumn{OUTCOL_TIME, OUTCOL_SIP, OUTCOL_PROTO,
            OUTCOL_BOTHPKTSRCVD, OUTCOL_BOTHPKTSSENT, OUTCOL_BOTHPKTSPERCENT,
            OUTCOL_BOTHBYTESRCVD, OUTCOL_BOTHBYTESSENT, OUTCOL_BOTHBYTESPERCENT,
        },
    },
    {
        "proto,time,sip",
        DIRECTION_BOTH,
        []OutputColumn{OUTCOL_TIME, OUTCOL_PROTO, OUTCOL_SIP,
            OUTCOL_BOTHPKTSRCVD, OUTCOL_BOTHPKTSSENT, OUTCOL_BOTHPKTSPERCENT,
            OUTCOL_BOTHBYTESRCVD, OUTCOL_BOTHBYTESSENT, OUTCOL_BOTHBYTESPERCENT,
        },
    },
    {
        "time,proto,iface,sip",
        DIRECTION_BOTH,
        []OutputColumn{OUTCOL_TIME, OUTCOL_IFACE, OUTCOL_PROTO, OUTCOL_SIP,
            OUTCOL_BOTHPKTSRCVD, OUTCOL_BOTHPKTSSENT, OUTCOL_BOTHPKTSPERCENT,
            OUTCOL_BOTHBYTESRCVD, OUTCOL_BOTHBYTESSENT, OUTCOL_BOTHBYTESPERCENT,
        },
    },
}

func TestColumns(t *testing.T) {
    for _, test := range columnsTests {
        attribs, hasAttrTime, hastAttrIface, err := goDB.ParseQueryType(test.queryType)
        if err != nil {
            t.Fatalf("Unexpected error: %s", err)
        }

        cols := columns(hasAttrTime, hastAttrIface, attribs, test.direction)

        if !reflect.DeepEqual(test.output, cols) {
            t.Fatalf("Expected %v, got %v", test.output, cols)
        }
    }
}

func TryLookupTest(t *testing.T) {
    m := map[string]string{
        "foo": "bar",
    }
    if "bar" != tryLookup(m, "foo") {
        t.Fatalf("Expected %s, got %s", "bar", tryLookup(m, "foo"))
    }
    if "hah" != tryLookup(m, "hah") {
        t.Fatalf("Expected %s, got %s", "hah", tryLookup(m, "hah"))
    }
}

var extractTestsEntry = Entry{
    goDB.ExtraKey{
        1455531929, // 02/15/2016 @ 10:25am (UTC)
        "eth1",
        goDB.Key{
            [16]byte{192, 168, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // 192.168.0.1
            [16]byte{10, 11, 12, 13, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // 10.11.12.13
            [2]byte{0xCB, 0xF1},                                          // 52209
            6,                                                            // TCP
            [2]byte{0, 141},                                              // Minecraft (category: Gaming)
        },
    },
    40 * 1024, // nBr
    20 * 1024, // nBs
    10,        // nPr
    3,         // nPs
}

var extractTests = []struct {
    format      Formatter
    ips2domains map[string]string
    totals      Counts
    outputs     [COUNT_OUTCOL]string // for each column
}{
    {
        TextFormatter{},
        map[string]string{},
        Counts{},
        [COUNT_OUTCOL]string{
            time.Unix(extractTestsEntry.k.Time, 0).Format("06-01-02 15:04:05"),
            "eth1",
            "192.168.0.1",
            "10.11.12.13",
            "52209",
            "TCP",
            "Minecraft", "Gaming",
            "10.00  ", "0.00", "40.00 kB", "0.00",
            "3.00  ", "0.00", "20.00 kB", "0.00",
            "13.00  ", "0.00", "60.00 kB", "0.00",
            "10.00  ", "3.00  ", "0.00", "40.00 kB", "20.00 kB", "0.00",
        },
    },
    {
        TextFormatter{},
        map[string]string{
            "192.168.0.1": "sip.example.com",
            "10.11.12.13": "dip.example.com",
        },
        Counts{},
        [COUNT_OUTCOL]string{
            time.Unix(extractTestsEntry.k.Time, 0).Format("06-01-02 15:04:05"),
            "eth1",
            "sip.example.com",
            "dip.example.com",
            "52209",
            "TCP",
            "Minecraft", "Gaming",
            "10.00  ", "0.00", "40.00 kB", "0.00",
            "3.00  ", "0.00", "20.00 kB", "0.00",
            "13.00  ", "0.00", "60.00 kB", "0.00",
            "10.00  ", "3.00  ", "0.00", "40.00 kB", "20.00 kB", "0.00",
        },
    },
    {
        TextFormatter{},
        map[string]string{},
        Counts{2 * 10, 3 * 3, 3 * 40 * 1024, 4 * 20 * 1024},
        [COUNT_OUTCOL]string{
            time.Unix(extractTestsEntry.k.Time, 0).Format("06-01-02 15:04:05"),
            "eth1",
            "192.168.0.1",
            "10.11.12.13",
            "52209",
            "TCP",
            "Minecraft", "Gaming",
            "10.00  ", "50.00", "40.00 kB", "33.33",
            "3.00  ", "33.33", "20.00 kB", "25.00",
            "13.00  ", "44.83", "60.00 kB", "30.00",
            "10.00  ", "3.00  ", "44.83", "40.00 kB", "20.00 kB", "30.00",
        },
    },
}

func TestExtract(t *testing.T) {
    for _, test := range extractTests {
        for col := OutputColumn(0); col < COUNT_OUTCOL; col++ {
            actual := extract(test.format, test.ips2domains, test.totals, extractTestsEntry, col)
            if test.outputs[col] != actual {
                t.Fatalf("Column %d: Expected '%s', got '%s'", col, test.outputs[col], actual)
            }
        }
    }
}

var extractTotalTests = []struct {
    format  Formatter
    totals  Counts
    outputs map[OutputColumn]string
}{
    {
        TextFormatter{},
        Counts{10, 3, 40 * 1024, 20 * 1024},
        map[OutputColumn]string{
            OUTCOL_INPKTS:        "10.00  ",
            OUTCOL_INBYTES:       "40.00 kB",
            OUTCOL_OUTPKTS:       "3.00  ",
            OUTCOL_OUTBYTES:      "20.00 kB",
            OUTCOL_SUMPKTS:       "13.00  ",
            OUTCOL_SUMBYTES:      "60.00 kB",
            OUTCOL_BOTHPKTSRCVD:  "10.00  ",
            OUTCOL_BOTHPKTSSENT:  "3.00  ",
            OUTCOL_BOTHBYTESRCVD: "40.00 kB",
            OUTCOL_BOTHBYTESSENT: "20.00 kB",
        },
    },
}

func TestExtractTotal(t *testing.T) {
    for _, test := range extractTotalTests {
        for col, expected := range test.outputs {
            actual := extractTotal(test.format, test.totals, col)
            if expected != actual {
                t.Fatalf("Column %d: Expected '%s', got '%s'", col, expected, actual)
            }
        }
    }
}

var printerAnsiTestsEntries = []Entry{
    {
        goDB.ExtraKey{
            1455531929, // 02/15/2016 @ 10:25am (UTC)
            "eth1",
            goDB.Key{
                [16]byte{172, 4, 12, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},  // 172.4.12.2
                [16]byte{10, 11, 12, 13, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // 10.11.12.13
                [2]byte{0x29, 0x45},                                          // 10565
                6,                                                            // TCP
                [2]byte{0, 141},                                              // Minecraft (category: Gaming)
            },
        },
        0,  // nBr
        5,  // nBs
        0,  // nPr
        2,  // nPs
    },
    {
        goDB.ExtraKey{
            1455531429, // 02/15/2016 @ 10:17am (UTC)
            "eth1",
            goDB.Key{
                [16]byte{172, 8, 12, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},  // 172.8.12.2
                [16]byte{10, 11, 12, 14, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // 10.11.12.14
                [2]byte{0x29, 0x45},                                          // 10565
                6,                                                            // TCP
                [2]byte{0, 141},                                              // Minecraft (category: Gaming)
            },
        },
        2094476019, // nBr
        262155310,  // nBs
        1578601,    // nPr
        81144,      // nPs
    },
}

var printerTestsEntries = []Entry{
    {
        goDB.ExtraKey{
            1455531929, // 02/15/2016 @ 10:25am (UTC)
            "eth1",
            goDB.Key{
                [16]byte{172, 4, 12, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},  // 172.4.12.2
                [16]byte{10, 11, 12, 13, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // 10.11.12.13
                [2]byte{0x29, 0x45},                                          // 10565
                6,                                                            // TCP
                [2]byte{0, 141},                                              // Minecraft (category: Gaming)
            },
        },
        7004484352, // nBr
        323451416,  // nBs
        4949136,    // nPr
        105893,     // nPs
    },
    {
        goDB.ExtraKey{
            1455531429, // 02/15/2016 @ 10:17am (UTC)
            "eth1",
            goDB.Key{
                [16]byte{172, 8, 12, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},  // 172.8.12.2
                [16]byte{10, 11, 12, 14, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // 10.11.12.14
                [2]byte{0x29, 0x45},                                          // 10565
                6,                                                            // TCP
                [2]byte{0, 141},                                              // Minecraft (category: Gaming)
            },
        },
        2094476019, // nBr
        262155310,  // nBs
        1578601,    // nPr
        81144,      // nPs
    },
}

type printerTest struct {
    sort                                                   SortOrder
    direction                                              Direction
    queryType                                              string
    ips2domains                                            map[string]string
    totalInPkts, totalOutPkts, totalInBytes, totalOutBytes uint64
    numFlows                                               int
    iface                                                  string
    entries                                                []Entry
    csvOutput                                              string
    jsonOutput                                             map[string]interface{}
    // We don't check the footer lines here but in TestTextTablePrinterFooter
    textOutputLines []string
    influxDbOutput  string
}

var printerAnsiTests = []printerTest{
    {   // direction in
        SORT_TRAFFIC,
        DIRECTION_IN,
        "sip,dip,dport,proto,l7proto",
        map[string]string{},
        12427491, 9790521, 10105124299, 2133066153,
        5,
        "eth1",
        printerAnsiTestsEntries,
        "",
        make(map[string]interface{}),
        []string{
            ``,
            `                                                              packets           bytes`,
            `         sip          dip  dport  proto    l7proto  category       in      %       in      %`,
            "  172.4.12.2  10.11.12.13  10565    TCP  Minecraft    Gaming   " + ANSI_SET_BOLD + "0.00  " + ANSI_RESET + "   " + ANSI_SET_BOLD + "0.00" + ANSI_RESET + "  " + ANSI_SET_BOLD + "0.00  B" + ANSI_RESET + "   " + ANSI_SET_BOLD + "0.00" + ANSI_RESET,
            `  172.8.12.2  10.11.12.14  10565    TCP  Minecraft    Gaming   1.58 M  12.70  1.95 GB  20.73`,
            `                                                                  ...             ...`,
            `                                                              12.43 M         9.41 GB`,
            ``,
            `Timespan / Interface`,
        },
        "",
    },
    {   // direction out
        SORT_TRAFFIC,
        DIRECTION_OUT,
        "sip,dip,dport,proto,l7proto",
        map[string]string{},
        12427491, 9790521, 10105124299, 2133066153,
        5,
        "eth1",
        printerAnsiTestsEntries,
        "",
        make(map[string]interface{}),
        []string{
            ``,
            `                                                              packets            bytes`,
            `         sip          dip  dport  proto    l7proto  category      out     %        out      %`,
            `  172.4.12.2  10.11.12.13  10565    TCP  Minecraft    Gaming   2.00    0.00    5.00  B   0.00`,
            `  172.8.12.2  10.11.12.14  10565    TCP  Minecraft    Gaming  81.14 k  0.83  250.01 MB  12.29`,
            ``,
            `                                                               9.79 M          1.99 GB`,
            ``,
        },
        "",
    },
}

var printerTests = []printerTest{
    {   // direction in
        SORT_TRAFFIC,
        DIRECTION_IN,
        "sip,dip,dport,proto,l7proto",
        map[string]string{},
        12427491, 9790521, 10105124299, 2133066153,
        5,
        "eth1",
        printerTestsEntries,
        `sip,dip,dport,proto,l7proto,category,packets,%,data vol.,%` + "\n" +
            `172.4.12.2,10.11.12.13,10565,TCP,Minecraft,Gaming,4949136,39.82,7004484352,69.32` + "\n" +
            `172.8.12.2,10.11.12.14,10565,TCP,Minecraft,Gaming,1578601,12.70,2094476019,20.73` + "\n" +
            `Overall packets,12427491` + "\n" +
            `Overall data volume (bytes),10105124299` + "\n" +
            `Sorting and flow direction,accumulated data volume (received only)` + "\n" +
            `Interface,eth1` + "\n",
        map[string]interface{}{
            "sip,dip,dport,proto,l7proto": []interface{}{
                map[string]interface{}{
                    "sip": "172.4.12.2", "dip": "10.11.12.13",
                    "dport": "10565", "proto": "TCP",
                    "l7proto": "Minecraft", "category": "Gaming",
                    "packets": 4949136.0, "packets_percent": 39.824096432658855, "bytes": 7004484352.0, "bytes_percent": 69.31616222368646,
                },
                map[string]interface{}{
                    "sip": "172.8.12.2", "dip": "10.11.12.14",
                    "dport": "10565", "proto": "TCP",
                    "l7proto": "Minecraft", "category": "Gaming",
                    "packets": 1578601.0, "packets_percent": 12.70249159705688, "bytes": 2094476019.0, "bytes_percent": 20.726870417687675,
                },
            },
            "status": "ok",
            "summary": map[string]interface{}{
                "interface":     "eth1",
                "total_packets": 12427491.0,
                "total_bytes":   10105124299.0,
            },
        },
        []string{
            ``,
            `                                                              packets           bytes`,
            `         sip          dip  dport  proto    l7proto  category       in      %       in      %`,
            `  172.4.12.2  10.11.12.13  10565    TCP  Minecraft    Gaming   4.95 M  39.82  6.52 GB  69.32`,
            `  172.8.12.2  10.11.12.14  10565    TCP  Minecraft    Gaming   1.58 M  12.70  1.95 GB  20.73`,
            `                                                                  ...             ...`,
            `                                                              12.43 M         9.41 GB`,
            ``,
            `Timespan / Interface`,
        },
        `goprobe_flows,category=Gaming,l7proto=Minecraft,proto=TCP sip=172.4.12.2,dip=10.11.12.13,dport=10565,packets=4949136i,bytes=7004484352i` + "\n" +
            `goprobe_flows,category=Gaming,l7proto=Minecraft,proto=TCP sip=172.8.12.2,dip=10.11.12.14,dport=10565,packets=1578601i,bytes=2094476019i` + "\n",
    },
    {   // direction out
        SORT_TRAFFIC,
        DIRECTION_OUT,
        "sip,dip,dport,proto,l7proto",
        map[string]string{},
        12427491, 9790521, 10105124299, 2133066153,
        2,
        "eth1",
        printerTestsEntries,
        `sip,dip,dport,proto,l7proto,category,packets,%,data vol.,%` + "\n" +
            `172.4.12.2,10.11.12.13,10565,TCP,Minecraft,Gaming,105893,1.08,323451416,15.16` + "\n" +
            `172.8.12.2,10.11.12.14,10565,TCP,Minecraft,Gaming,81144,0.83,262155310,12.29` + "\n" +
            `Overall packets,9790521` + "\n" +
            `Overall data volume (bytes),2133066153` + "\n" +
            `Sorting and flow direction,accumulated data volume (sent only)` + "\n" +
            `Interface,eth1` + "\n",
        map[string]interface{}{
            "sip,dip,dport,proto,l7proto": []interface{}{
                map[string]interface{}{
                    "sip": "172.4.12.2", "dip": "10.11.12.13",
                    "dport": "10565", "proto": "TCP",
                    "l7proto": "Minecraft", "category": "Gaming",
                    "packets": 105893.0, "packets_percent": 1.0815869758105825, "bytes": 323451416.0, "bytes_percent": 15.163684236660428,
                },
                map[string]interface{}{
                    "sip": "172.8.12.2", "dip": "10.11.12.14",
                    "dport": "10565", "proto": "TCP",
                    "l7proto": "Minecraft", "category": "Gaming",
                    "packets": 81144.0, "packets_percent": 0.8288016541714175, "bytes": 262155310.0, "bytes_percent": 12.290069374140034,
                },
            },
            "status": "ok",
            "summary": map[string]interface{}{
                "interface":     "eth1",
                "total_packets": 9790521.0,
                "total_bytes":   2133066153.0,
            },
        },
        []string{
            ``,
            `                                                               packets            bytes`,
            `         sip          dip  dport  proto    l7proto  category       out     %        out      %`,
            `  172.4.12.2  10.11.12.13  10565    TCP  Minecraft    Gaming  105.89 k  1.08  308.47 MB  15.16`,
            `  172.8.12.2  10.11.12.14  10565    TCP  Minecraft    Gaming   81.14 k  0.83  250.01 MB  12.29`,
            ``,
            `                                                                9.79 M          1.99 GB`,
            ``,
        },
        `goprobe_flows,category=Gaming,l7proto=Minecraft,proto=TCP sip=172.4.12.2,dip=10.11.12.13,dport=10565,packets=105893i,bytes=323451416i` + "\n" +
            `goprobe_flows,category=Gaming,l7proto=Minecraft,proto=TCP sip=172.8.12.2,dip=10.11.12.14,dport=10565,packets=81144i,bytes=262155310i` + "\n",
    },
    {   // direction both
        SORT_TRAFFIC,
        DIRECTION_BOTH,
        "sip,dip,dport,proto,l7proto",
        map[string]string{},
        12427491, 9790521, 10105124299, 2133066153,
        5,
        "eth1",
        printerTestsEntries,
        `sip,dip,dport,proto,l7proto,category,packets received,packets sent,%,data vol. received,data vol. sent,%` + "\n" +
            `172.4.12.2,10.11.12.13,10565,TCP,Minecraft,Gaming,4949136,105893,22.75,7004484352,323451416,59.88` + "\n" +
            `172.8.12.2,10.11.12.14,10565,TCP,Minecraft,Gaming,1578601,81144,7.47,2094476019,262155310,19.26` + "\n" +
            `Received packets,12427491` + "\n" +
            `Sent packets,9790521` + "\n" +
            `Received data volume (bytes),10105124299` + "\n" +
            `Sent data volume (bytes),2133066153` + "\n" +
            `Sorting and flow direction,accumulated data volume (sent and received)` + "\n" +
            `Interface,eth1` + "\n",
        map[string]interface{}{
            "sip,dip,dport,proto,l7proto": []interface{}{
                map[string]interface{}{
                    "sip": "172.4.12.2", "dip": "10.11.12.13",
                    "dport": "10565", "proto": "TCP",
                    "l7proto": "Minecraft", "category": "Gaming",
                    "packets_rcvd": 4949136.0, "packets_sent": 105893.0, "packets_percent": 22.75194108275754, "bytes_rcvd": 7004484352.0, "bytes_sent": 323451416.0, "bytes_percent": 59.87760851362178,
                },
                map[string]interface{}{
                    "sip": "172.8.12.2", "dip": "10.11.12.14",
                    "dport": "10565", "proto": "TCP",
                    "l7proto": "Minecraft", "category": "Gaming",
                    "packets_rcvd": 1578601.0, "packets_sent": 81144.0, "packets_percent": 7.470267816940598, "bytes_rcvd": 2094476019.0, "bytes_sent": 262155310.0, "bytes_percent": 19.25637077019726,
                },
            },
            "status": "ok",
            "summary": map[string]interface{}{
                "interface":          "eth1",
                "total_packets_rcvd": 12427491.0,
                "total_packets_sent": 9790521.0,
                "total_bytes_rcvd":   10105124299.0,
                "total_bytes_sent":   2133066153.0,
            },
        },
        []string{
            ``,
            `                                                              packets   packets           bytes      bytes`,
            `         sip          dip  dport  proto    l7proto  category       in       out      %       in        out      %`,
            `  172.4.12.2  10.11.12.13  10565    TCP  Minecraft    Gaming   4.95 M  105.89 k  22.75  6.52 GB  308.47 MB  59.88`,
            `  172.8.12.2  10.11.12.14  10565    TCP  Minecraft    Gaming   1.58 M   81.14 k   7.47  1.95 GB  250.01 MB  19.26`,
            `                                                                  ...       ...             ...        ...`,
            `                                                              12.43 M    9.79 M         9.41 GB    1.99 GB`,
            ``,
            `     Totals:                                                            22.22 M                   11.40 GB`,
            ``,
        },
        `goprobe_flows,category=Gaming,l7proto=Minecraft,proto=TCP sip=172.4.12.2,dip=10.11.12.13,dport=10565,packets_rcvd=4949136i,packets_sent=105893i,bytes_rcvd=7004484352i,bytes_sent=323451416i` + "\n" +
            `goprobe_flows,category=Gaming,l7proto=Minecraft,proto=TCP sip=172.8.12.2,dip=10.11.12.14,dport=10565,packets_rcvd=1578601i,packets_sent=81144i,bytes_rcvd=2094476019i,bytes_sent=262155310i` + "\n",
    },
    {   // direction sum
        SORT_TRAFFIC,
        DIRECTION_SUM,
        "sip,dip,dport,proto,l7proto",
        map[string]string{},
        12427491, 9790521, 10105124299, 2133066153,
        5,
        "eth1",
        printerTestsEntries,
        `sip,dip,dport,proto,l7proto,category,packets,%,data vol.,%` + "\n" +
            `172.4.12.2,10.11.12.13,10565,TCP,Minecraft,Gaming,5055029,22.75,7327935768,59.88` + "\n" +
            `172.8.12.2,10.11.12.14,10565,TCP,Minecraft,Gaming,1659745,7.47,2356631329,19.26` + "\n" +
            `Overall packets,22218012` + "\n" +
            `Overall data volume (bytes),12238190452` + "\n" +
            `Sorting and flow direction,accumulated data volume (sent and received)` + "\n" +
            `Interface,eth1` + "\n",
        map[string]interface{}{
            "sip,dip,dport,proto,l7proto": []interface{}{
                map[string]interface{}{
                    "sip": "172.4.12.2", "dip": "10.11.12.13",
                    "dport": "10565", "proto": "TCP",
                    "l7proto": "Minecraft", "category": "Gaming",
                    "packets": 5055029.0, "packets_percent": 22.75194108275754, "bytes": 7327935768.0, "bytes_percent": 59.87760851362178,
                },
                map[string]interface{}{
                    "sip": "172.8.12.2", "dip": "10.11.12.14",
                    "dport": "10565", "proto": "TCP",
                    "l7proto": "Minecraft", "category": "Gaming",
                    "packets": 1659745.0, "packets_percent": 7.470267816940598, "bytes": 2356631329.0, "bytes_percent": 19.25637077019726,
                },
            },
            "status": "ok",
            "summary": map[string]interface{}{
                "interface":     "eth1",
                "total_packets": 22218012.0,
                "total_bytes":   12238190452.0,
            },
        },
        []string{
            ``,
            `                                                              packets            bytes`,
            `         sip          dip  dport  proto    l7proto  category   in+out      %    in+out      %`,
            `  172.4.12.2  10.11.12.13  10565    TCP  Minecraft    Gaming   5.06 M  22.75   6.82 GB  59.88`,
            `  172.8.12.2  10.11.12.14  10565    TCP  Minecraft    Gaming   1.66 M   7.47   2.19 GB  19.26`,
            `                                                                  ...              ...`,
            `                                                              22.22 M         11.40 GB`,
            ``,
        },
        `goprobe_flows,category=Gaming,l7proto=Minecraft,proto=TCP sip=172.4.12.2,dip=10.11.12.13,dport=10565,packets=5055029i,bytes=7327935768i` + "\n" +
            `goprobe_flows,category=Gaming,l7proto=Minecraft,proto=TCP sip=172.8.12.2,dip=10.11.12.14,dport=10565,packets=1659745i,bytes=2356631329i` + "\n",
    },
    {   // with time attribute
        SORT_TRAFFIC,
        DIRECTION_SUM,
        "time,sip,dip",
        map[string]string{},
        12427491, 9790521, 10105124299, 2133066153,
        5,
        "eth1",
        printerTestsEntries,
        `time,sip,dip,packets,%,data vol.,%` + "\n" +
            `1455531929,172.4.12.2,10.11.12.13,5055029,22.75,7327935768,59.88` + "\n" +
            `1455531429,172.8.12.2,10.11.12.14,1659745,7.47,2356631329,19.26` + "\n" +
            `Overall packets,22218012` + "\n" +
            `Overall data volume (bytes),12238190452` + "\n" +
            `Sorting and flow direction,accumulated data volume (sent and received)` + "\n" +
            `Interface,eth1` + "\n",
        map[string]interface{}{
            "time,sip,dip": []interface{}{
                map[string]interface{}{
                    "time": "1455531929",
                    "sip":  "172.4.12.2", "dip": "10.11.12.13",
                    "packets": 5055029.0, "packets_percent": 22.75194108275754, "bytes": 7327935768.0, "bytes_percent": 59.87760851362178,
                },
                map[string]interface{}{
                    "time": "1455531429",
                    "sip":  "172.8.12.2", "dip": "10.11.12.14",
                    "packets": 1659745.0, "packets_percent": 7.470267816940598, "bytes": 2356631329.0, "bytes_percent": 19.25637077019726,
                },
            },
            "status": "ok",
            "summary": map[string]interface{}{
                "interface":     "eth1",
                "total_packets": 22218012.0,
                "total_bytes":   12238190452.0,
            },
        },
        []string{
            ``,
            `                                              packets            bytes`,
            `               time         sip          dip   in+out      %    in+out      %`,
            `  ` + TextFormatter{}.Time(1455531929) + `  172.4.12.2  10.11.12.13   5.06 M  22.75   6.82 GB  59.88`,
            `  ` + TextFormatter{}.Time(1455531429) + `  172.8.12.2  10.11.12.14   1.66 M   7.47   2.19 GB  19.26`,
            `                                                  ...              ...`,
            `                                              22.22 M         11.40 GB`,
            ``,
        },
        `goprobe_flows sip=172.4.12.2,dip=10.11.12.13,packets=5055029i,bytes=7327935768i 1455531929000000000` + "\n" +
            `goprobe_flows sip=172.8.12.2,dip=10.11.12.14,packets=1659745i,bytes=2356631329i 1455531429000000000` + "\n",
    },
    {   // with iface attribute
        SORT_TRAFFIC,
        DIRECTION_SUM,
        "iface,sip,dip",
        map[string]string{},
        12427491, 9790521, 10105124299, 2133066153,
        5,
        "eth1",
        printerTestsEntries,
        `iface,sip,dip,packets,%,data vol.,%` + "\n" +
            `eth1,172.4.12.2,10.11.12.13,5055029,22.75,7327935768,59.88` + "\n" +
            `eth1,172.8.12.2,10.11.12.14,1659745,7.47,2356631329,19.26` + "\n" +
            `Overall packets,22218012` + "\n" +
            `Overall data volume (bytes),12238190452` + "\n" +
            `Sorting and flow direction,accumulated data volume (sent and received)` + "\n" +
            `Interface,eth1` + "\n",
        map[string]interface{}{
            "iface,sip,dip": []interface{}{
                map[string]interface{}{
                    "iface": "eth1",
                    "sip":   "172.4.12.2", "dip": "10.11.12.13",
                    "packets": 5055029.0, "packets_percent": 22.75194108275754, "bytes": 7327935768.0, "bytes_percent": 59.87760851362178,
                },
                map[string]interface{}{
                    "iface": "eth1",
                    "sip":   "172.8.12.2", "dip": "10.11.12.14",
                    "packets": 1659745.0, "packets_percent": 7.470267816940598, "bytes": 2356631329.0, "bytes_percent": 19.25637077019726,
                },
            },
            "status": "ok",
            "summary": map[string]interface{}{
                "interface":     "eth1",
                "total_packets": 22218012.0,
                "total_bytes":   12238190452.0,
            },
        },
        []string{
            ``,
            `                                  packets            bytes`,
            `  iface         sip          dip   in+out      %    in+out      %`,
            `   eth1  172.4.12.2  10.11.12.13   5.06 M  22.75   6.82 GB  59.88`,
            `   eth1  172.8.12.2  10.11.12.14   1.66 M   7.47   2.19 GB  19.26`,
            `                                      ...              ...`,
            `                                  22.22 M         11.40 GB`,
            ``,
        },
        `goprobe_flows,iface=eth1 sip=172.4.12.2,dip=10.11.12.13,packets=5055029i,bytes=7327935768i` + "\n" +
            `goprobe_flows,iface=eth1 sip=172.8.12.2,dip=10.11.12.14,packets=1659745i,bytes=2356631329i` + "\n",
    },
    {   // reverse DNS
        SORT_TRAFFIC,
        DIRECTION_IN,
        "sip,dip,dport,proto,l7proto",
        map[string]string{
            "172.4.12.2":  "da-sh.open.ch",
            "10.11.12.14": "www.inf.ethz.ch",
        },
        12427491, 9790521, 10105124299, 2133066153,
        5,
        "eth1",
        printerTestsEntries,
        `sip,dip,dport,proto,l7proto,category,packets,%,data vol.,%` + "\n" +
            `da-sh.open.ch,10.11.12.13,10565,TCP,Minecraft,Gaming,4949136,39.82,7004484352,69.32` + "\n" +
            `172.8.12.2,www.inf.ethz.ch,10565,TCP,Minecraft,Gaming,1578601,12.70,2094476019,20.73` + "\n" +
            `Overall packets,12427491` + "\n" +
            `Overall data volume (bytes),10105124299` + "\n" +
            `Sorting and flow direction,accumulated data volume (received only)` + "\n" +
            `Interface,eth1` + "\n",
        map[string]interface{}{
            "sip,dip,dport,proto,l7proto": []interface{}{
                map[string]interface{}{
                    "sip": "da-sh.open.ch", "dip": "10.11.12.13",
                    "dport": "10565", "proto": "TCP",
                    "l7proto": "Minecraft", "category": "Gaming",
                    "packets": 4949136.0, "packets_percent": 39.824096432658855, "bytes": 7004484352.0, "bytes_percent": 69.31616222368646,
                },
                map[string]interface{}{
                    "sip": "172.8.12.2", "dip": "www.inf.ethz.ch",
                    "dport": "10565", "proto": "TCP",
                    "l7proto": "Minecraft", "category": "Gaming",
                    "packets": 1578601.0, "packets_percent": 12.70249159705688, "bytes": 2094476019.0, "bytes_percent": 20.726870417687675,
                },
            },
            "status": "ok",
            "summary": map[string]interface{}{
                "interface":     "eth1",
                "total_packets": 12427491.0,
                "total_bytes":   10105124299.0,
            },
        },
        []string{
            ``,
            `                                                                     packets           bytes`,
            `            sip              dip  dport  proto    l7proto  category       in      %       in      %`,
            `  da-sh.open.ch      10.11.12.13  10565    TCP  Minecraft    Gaming   4.95 M  39.82  6.52 GB  69.32`,
            `     172.8.12.2  www.inf.ethz.ch  10565    TCP  Minecraft    Gaming   1.58 M  12.70  1.95 GB  20.73`,
            `                                                                         ...             ...`,
            `                                                                     12.43 M         9.41 GB`,
            ``,
        },
        `goprobe_flows,category=Gaming,l7proto=Minecraft,proto=TCP sip=da-sh.open.ch,dip=10.11.12.13,dport=10565,packets=4949136i,bytes=7004484352i` + "\n" +
            `goprobe_flows,category=Gaming,l7proto=Minecraft,proto=TCP sip=172.8.12.2,dip=www.inf.ethz.ch,dport=10565,packets=1578601i,bytes=2094476019i` + "\n",
    },
}

func testCSVTablePrinter(t *testing.T, test printerTest, b basePrinter, buf *bytes.Buffer) {
    c := NewCSVTablePrinter(b)
    for _, entry := range test.entries {
        c.AddRow(entry)
    }
    c.Footer("", time.Now(), time.Now(), time.Duration(0), time.Duration(0))
    if err := c.Print(); err != nil {
        t.Fatalf("Unexpected error during Print(): %s", err)
    }

    bufbytes := buf.Bytes()
    if !bytes.Equal([]byte(test.csvOutput), bufbytes) {
        t.Fatalf("Expected output:\n%s\nActual output:\n%s\n", test.csvOutput, bufbytes)
    }
}

func testJSONTablePrinter(t *testing.T, test printerTest, b basePrinter, buf *bytes.Buffer) {
    j := NewJSONTablePrinter(b, test.queryType)
    for _, entry := range test.entries {
        j.AddRow(entry)
    }
    j.Footer("", time.Now(), time.Now(), time.Duration(0), time.Duration(0))
    if err := j.Print(); err != nil {
        t.Fatalf("Unexpected error during Print(): %s", err)
    }

    bufbytes := buf.Bytes()

    var actual map[string]interface{}
    json.NewDecoder(buf).Decode(&actual)

    // Make sure that actual has a "ext_ips" entry. Then copy it from the actual output,
    // because it is machine dependent and hard to test for.
    if _, exists := actual["ext_ips"]; !exists {
        t.Fatalf("'ext_ips' entry missing from\n%s", bufbytes)
    }
    test.jsonOutput["ext_ips"] = actual["ext_ips"]

    if !reflect.DeepEqual(test.jsonOutput, actual) {
        expected, _ := json.Marshal(test.jsonOutput)
        t.Fatalf("Expected output:\n%s\nActual output:\n%s\n", expected, bufbytes)
    }
}

func testTextTablePrinter(t *testing.T, test printerTest, b basePrinter, buf *bytes.Buffer) {
    p := NewTextTablePrinter(b, test.numFlows, time.Duration(0))
    for _, entry := range test.entries {
        p.AddRow(entry)
    }
    p.Footer("", time.Now(), time.Now(), time.Duration(0), time.Duration(0))
    if err := p.Print(); err != nil {
        t.Fatalf("Unexpected error during Print(): %s", err)
    }

    for i, expectedLine := range test.textOutputLines {
        actualLine, err := buf.ReadString('\n')
        if err != nil {
            t.Fatalf("Unexpected error in line %d: %s", i, err)
        }
        if !strings.HasPrefix(actualLine, expectedLine) {
            t.Log(buf.String())
            t.Fatalf("Line %d is wrong. Expected line: `%s` Actual line: `%s`", i, expectedLine, actualLine)
        }
    }
}

func testInfluxDBTablePrinter(t *testing.T, test printerTest, b basePrinter, buf *bytes.Buffer) {
    i := NewInfluxDBTablePrinter(b)
    for _, entry := range test.entries {
        i.AddRow(entry)
    }
    i.Footer("", time.Now(), time.Now(), time.Duration(0), time.Duration(0)) // footer is irrelevant for influxdb
    if err := i.Print(); err != nil {
        t.Fatalf("Unexpected error during Print(): %s", err)
    }

    bufbytes := buf.Bytes()
    if !bytes.Equal([]byte(test.influxDbOutput), bufbytes) {
        t.Fatalf("Expected output:\n%s\nActual output:\n%s\n", test.influxDbOutput, bufbytes)
    }
}

func TestPrinters(t *testing.T) {
    bp := func(test printerTest) basePrinter {
        attribs, hasAttrTime, hasAttrIface, err := goDB.ParseQueryType(test.queryType)
        if err != nil {
            t.Fatalf("Unexpected error: %s", err)
        }
        b := makeBasePrinter(
            test.sort,
            hasAttrTime, hasAttrIface,
            test.direction,
            attribs,
            test.ips2domains,
            test.totalInPkts, test.totalOutPkts, test.totalInBytes, test.totalOutBytes,
            test.iface,
        )
        return b
    }

    for _, test := range printerTests {
        buf := &bytes.Buffer{}
        output = buf
        testCSVTablePrinter(t, test, bp(test), buf)

        buf = &bytes.Buffer{}
        output = buf
        testJSONTablePrinter(t, test, bp(test), buf)

        buf = &bytes.Buffer{}
        output = buf
        testTextTablePrinter(t, test, bp(test), buf)

        buf = &bytes.Buffer{}
        output = buf
        testInfluxDBTablePrinter(t, test, bp(test), buf)

    }

    // set output back to os.Stdout in case other tests depend on it.
    output = os.Stdout
}

func TestAnsiPrinters(t *testing.T) {
    bp := func(test printerTest) basePrinter {
        attribs, hasAttrTime, hasAttrIface, err := goDB.ParseQueryType(test.queryType)
        if err != nil {
            t.Fatalf("Unexpected error: %s", err)
        }
        b := makeBasePrinter(
            test.sort,
            hasAttrTime, hasAttrIface,
            test.direction,
            attribs,
            test.ips2domains,
            test.totalInPkts, test.totalOutPkts, test.totalInBytes, test.totalOutBytes,
            test.iface,
        )
        return b
    }

    for _, test := range printerAnsiTests {
        buf := &bytes.Buffer{}
        output = buf
        testTextTablePrinter(t, test, bp(test), buf)

    }

    // set output back to os.Stdout in case other tests depend on it.
    output = os.Stdout
}

var textTablePrinterFooterTests = []struct {
    sort                                           SortOrder
    hasAttrTime                                    bool
    hasAttrIface                                   bool
    direction                                      Direction
    numFlows                                       int
    iface                                          string
    conditional                                    string
    spanFirst, spanLast                            time.Time
    queryDuration, resolveDuration, resolveTimeout time.Duration
    outputRegex                                    string
}{
    {
        SORT_TRAFFIC,
        false,
        false,
        DIRECTION_BOTH,
        1270,
        "eth17",
        "",
        time.Unix(1455522462, 0), time.Unix(1455622462, 0),
        17 * time.Second, 0, 2 * time.Second,
        `\nTimespan \/ Interface : \[` + time.Unix(1455522462, 0).Format("2006-01-02 15:04:05") + `, ` + time.Unix(1455622462, 0).Format("2006-01-02 15:04:05") + `\] \/ eth17\n` +
            `Sorted by            : accumulated data volume \(sent and received\)\n` +
            `Query stats          : 1.27 k hits in 17.0s\n`,
    },
    {
        SORT_PACKETS,
        false,
        true,
        DIRECTION_OUT,
        1270,
        "t4_1232",
        "",
        time.Unix(1455522462, 0), time.Unix(1455622462, 0),
        17 * time.Second, 18*time.Millisecond + 500*time.Microsecond, 2 * time.Second,
        `\nTimespan \/ Interface : \[` + time.Unix(1455522462, 0).Format("2006-01-02 15:04:05") + `, ` + time.Unix(1455622462, 0).Format("2006-01-02 15:04:05") + `\] \/ t4_1232\n` +
            `Sorted by            : accumulated packets \(sent only\)\n` +
            `Reverse DNS stats    : RDNS took 18ms, timeout was 2\.0s\n` +
            `Query stats          : 1.27 k hits in 17.0s\n`,
    },
    {
        SORT_TIME,
        true,
        true,
        DIRECTION_SUM,
        92270,
        "eth17",
        "sip = 10.0.0.1 | dip = open.ch",
        time.Unix(1455522462, 0), time.Unix(1455622462, 0),
        17 * time.Millisecond, 18*time.Millisecond + 500*time.Microsecond, 500 * time.Millisecond,
        `\nTimespan \/ Interface : \[` + time.Unix(1455522462, 0).Format("2006-01-02 15:04:05") + `, ` + time.Unix(1455622462, 0).Format("2006-01-02 15:04:05") + `\] \/ eth17\n` +
            `Sorted by            : first packet time\n` +
            `Reverse DNS stats    : RDNS took 18ms, timeout was 500ms\n` +
            `Query stats          : 92.27 k hits in 17ms\n` +
            `Conditions:          : sip = 10.0.0.1 \| dip = open.ch\n`,
    },
}

func TestTextTablePrinterFooter(t *testing.T) {
    for _, test := range textTablePrinterFooterTests {
        buf := &bytes.Buffer{}
        output = buf

        b := makeBasePrinter(
            test.sort,
            test.hasAttrTime, test.hasAttrIface,
            test.direction,
            nil,
            nil,
            0, 0, 0, 0,
            test.iface,
        )
        p := NewTextTablePrinter(b, test.numFlows, test.resolveTimeout)

        p.Footer(test.conditional, test.spanFirst, test.spanLast, test.queryDuration, test.resolveDuration)
        if err := p.Print(); err != nil {
            t.Fatalf("Unexpected error: %s", err)
        }

        bufbytes := buf.Bytes()
        if !regexp.MustCompile(test.outputRegex).Match(bufbytes) {
            t.Fatalf("Output doesn't match regexp. Output: \n%s`", bufbytes)
        }
    }

    // set output back to os.Stdout in case other tests depend on it.
    output = os.Stdout
}

var textFormatterSizeTests = []struct {
    size   uint64
    output string
}{
    {0, ANSI_SET_BOLD + "0.00  B" + ANSI_RESET},
    {125, "125.00  B"},
    {11*1024 + 15, "11.01 kB"},
    {(11*1024 + 15) * 1024, "11.01 MB"},
    {(11*1024 + 15) * 1024 * 1024, "11.01 GB"},
    {(11*1024 + 15) * 1024 * 1024 * 1024, "11.01 TB"},
    {(11*1024 + 15) * 1024 * 1024 * 1024 * 1024, "11.01 PB"},
}

func TestTextFormatterSize(t *testing.T) {
    for _, test := range textFormatterSizeTests {
        if (test.output != TextFormatter{}.Size(test.size)) {
            t.Fatalf("Expected: %s Got: %s", test.output, TextFormatter{}.Size(test.size))
        }
    }
}

var textFormatterCountTests = []struct {
    size   uint64
    output string
}{
    {0, ANSI_SET_BOLD + "0.00  " + ANSI_RESET},
    {125, "125.00  "},
    {1250, "1.25 k"},
    {1250000, "1.25 M"},
    {1250000000, "1.25 G"},
    {1250000000000, "1.25 T"},
    {1250000000000000, "1.25 P"},
}

func TestTextFormatterCount(t *testing.T) {
    for _, test := range textFormatterCountTests {
        if (test.output != TextFormatter{}.Count(test.size)) {
            t.Fatalf("Expected: %s Got: %s", test.output, TextFormatter{}.Count(test.size))
        }
    }
}

var influxDBFormatterStringTests = []struct {
    in, out string
}{
    {`hello`, `hello`},
    {`hel lo`, `hel\ lo`},
    {`hello,asd`, `hello\,asd`},
    {`hello\,!`, `hello\\\,!`},
}

func TestInfluxDBFormatterString(t *testing.T) {
    for _, test := range influxDBFormatterStringTests {
        if (test.out != InfluxDBFormatter{}.String(test.in)) {
            t.Fatalf("Expected: %s Got: %s", test.out, InfluxDBFormatter{}.String(test.in))
        }
    }
}
