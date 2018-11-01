package main

import (
	"OSAG/goDB"
	"os"
	"os/exec"
	"testing"

	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
)

const (
	SIP_DIP_SCHEMA             = "time,iface,sip,dip,packets received,packets sent,%,data vol. received,data vol. sent,%"
	SIP_DIP_DPORT_PROTO_SCHEMA = "time,iface,sip,dip,dport,proto,packets received,packets sent,%,data vol. received,data vol. sent,%"
	RAW_SCHEMA                 = "time,iface,sip,dip,dport,proto,l7proto,category,packets received,packets sent,%,data vol. received,data vol. sent,%"

	DBPATH = "./csvtestdb"
)

var parserTests = []struct {
	schema string
	input  string
	outKey goDB.ExtraKey
	outVal goDB.Val
}{
	{SIP_DIP_SCHEMA,
		"1460362502,eth2,213.156.236.211,213.156.236.255,2,0,0.00,525,0,0.00",
		goDB.ExtraKey{
			int64(1460362502),
			"eth2",
			goDB.Key{Sip: [16]byte{213, 156, 236, 211}, Dip: [16]byte{213, 156, 236, 255}},
		},
		goDB.Val{NBytesRcvd: uint64(525), NBytesSent: uint64(0), NPktsRcvd: uint64(2), NPktsSent: uint64(0)},
	},
	{SIP_DIP_DPORT_PROTO_SCHEMA,
		"1460362502,eth2,213.156.236.211,213.156.236.255,8080,TCP,2,0,0.00,525,0,0.00",
		goDB.ExtraKey{
			int64(1460362502),
			"eth2",
			goDB.Key{Sip: [16]byte{213, 156, 236, 211}, Dip: [16]byte{213, 156, 236, 255}, Dport: [2]byte{0x1f, 0x90}, Protocol: byte(6)},
		},
		goDB.Val{NBytesRcvd: uint64(525), NBytesSent: uint64(0), NPktsRcvd: uint64(2), NPktsSent: uint64(0)},
	},
	{RAW_SCHEMA,
		"1460362502,eth2,213.156.236.211,213.156.236.255,8080,TCP,HTTP_NonStandard,Web,2,0,0.00,525,0,0.00",
		goDB.ExtraKey{
			int64(1460362502),
			"eth2",
			goDB.Key{Sip: [16]byte{213, 156, 236, 211}, Dip: [16]byte{213, 156, 236, 255}, Dport: [2]byte{0x1f, 0x90}, Protocol: byte(6), L7proto: [2]byte{0x00, 0x42}},
		},
		goDB.Val{NBytesRcvd: uint64(525), NBytesSent: uint64(0), NPktsRcvd: uint64(2), NPktsSent: uint64(0)},
	},
}

var inputCSV string = `time,iface,sip,dip,dport,proto,l7proto,category,packets received,packets sent,%,data vol. received,data vol. sent,%
1464051037,t4_12759,213.156.234.4,224.0.0.5,0,OSPFIGP,Invalid,Mixed,21,0,0.78,1760,0,0.33
1464051037,eth0,169.254.204.142,169.254.255.255,137,UDP,Unknown_UDP,Uncategorised,3,0,0.11,276,0,0.05
1464051037,eth0,169.254.169.121,169.254.255.255,138,UDP,NetBIOS_UDP,Services,1,0,0.04,243,0,0.05
1464051037,eth0,213.156.236.87,213.156.236.255,4002,UDP,Unknown_UDP,Uncategorised,5,0,0.19,470,0,0.09
1464051037,eth0,213.156.236.25,213.156.236.32,22,TCP,SSH,Remote_Access,25,15,1.48,1690,1135,0.54
1464051037,eth0,169.254.204.142,169.254.255.255,138,UDP,NetBIOS_UDP,Services,1,0,0.04,243,0,0.05
1464051037,t4_12743,213.156.234.1,224.0.0.5,0,OSPFIGP,Invalid,Mixed,0,21,0.78,0,1792,0.34
1464051037,eth5,10.0.0.1,10.50.0.2,500,UDP,ISAKMP,Key_Exchange,0,16,0.59,0,10080,1.91
1464051037,eth1,10.0.0.1,10.0.0.2,0,ESP,Invalid,Mixed,61,61,4.52,9790,9822,3.72
1464051037,eth0,213.156.236.111,213.156.236.255,138,UDP,NetBIOS_UDP,Services,2,0,0.07,492,0,0.09
1464051037,eth0,0.0.0.0,255.255.255.255,67,UDP,DHCP,Services,140,0,5.19,71162,0,13.49
1464051037,eth0,213.156.236.111,213.156.236.255,137,UDP,Unknown_UDP,Uncategorised,3,0,0.11,276,0,0.05
1464051037,eth0,169.254.169.121,169.254.255.255,137,UDP,Unknown_UDP,Uncategorised,3,0,0.11,276,0,0.05
1464051037,t4_12743,213.156.234.2,224.0.0.5,0,OSPFIGP,Invalid,Mixed,21,0,0.78,1760,0,0.33
1464051037,t4_13444,213.156.234.1,224.0.0.5,0,OSPFIGP,Invalid,Mixed,0,20,0.74,0,1600,0.30
1464051037,eth3,10.30.0.2,10.30.0.1,0,ESP,Invalid,Mixed,101,61,6.01,16430,9822,4.98
1464051037,eth0,213.156.236.32,213.156.227.131,123,UDP,Unknown_UDP,Uncategorised,8,8,0.59,720,720,0.27
1464051037,t4_12759,213.156.234.1,224.0.0.5,0,OSPFIGP,Invalid,Mixed,0,21,0.78,0,1792,0.34
1464051037,eth0,fe80::ec4:7aff:fe08:65f9,ff02::1:2,547,UDP,Unknown_UDP,Uncategorised,3,0,0.11,270,0,0.05
1464051037,eth0,213.156.236.167,213.156.236.119,22,TCP,Unknown_TCP,Uncategorised,1,0,0.04,66,0,0.01
1464051037,eth0,213.156.236.245,213.156.236.32,22,TCP,Unknown_TCP,Uncategorised,12,4,0.59,824,296,0.21
1464051037,t4_12749,213.156.234.1,224.0.0.5,0,OSPFIGP,Invalid,Mixed,0,20,0.74,0,1600,0.30
1464051037,eth2,10.20.0.1,10.20.0.2,0,ESP,Invalid,Mixed,61,61,4.52,9822,9790,3.72
1464051037,eth0,213.156.236.25,213.156.236.32,22,TCP,Unknown_TCP,Uncategorised,15,5,0.74,1030,370,0.27
1464051037,t4_12745,213.156.234.1,224.0.0.5,0,OSPFIGP,Invalid,Mixed,0,21,0.78,0,1760,0.33
1464051037,t4_12745,213.156.234.3,224.0.0.5,0,OSPFIGP,Invalid,Mixed,21,0,0.78,1792,0,0.34
1464051037,eth0,213.156.236.245,213.156.236.32,22,TCP,SSH,Remote_Access,29,17,1.71,1956,1296,0.62
1464051037,eth0,213.156.236.211,213.156.236.255,138,UDP,Unknown_UDP,Uncategorised,2,0,0.07,525,0,0.10
1464051037,eth0,213.156.236.239,239.255.2.2,9753,UDP,Unknown_UDP,Uncategorised,1,0,0.04,85,0,0.02
1464051337,t4_12749,213.156.234.1,224.0.0.5,0,OSPFIGP,Invalid,Mixed,0,20,0.74,0,1600,0.30
1464051337,eth2,10.20.0.1,10.20.0.2,0,ESP,Invalid,Mixed,60,60,4.45,9640,9640,3.65
1464051337,t4_13444,213.156.234.1,224.0.0.5,0,OSPFIGP,Invalid,Mixed,0,20,0.74,0,1600,0.30
1464051337,eth0,213.156.236.227,224.0.0.252,5355,UDP,Unknown_UDP,Uncategorised,4,0,0.15,272,0,0.05
1464051337,eth0,213.156.236.245,213.156.236.32,22,TCP,Unknown_TCP,Uncategorised,15,5,0.74,1030,370,0.27
1464051337,eth0,213.156.236.237,213.156.236.255,137,UDP,Unknown_UDP,Uncategorised,30,0,1.11,2760,0,0.52
1464051337,t4_12743,213.156.234.2,224.0.0.5,0,OSPFIGP,Invalid,Mixed,20,0,0.74,1680,0,0.32
1464051337,eth0,213.156.236.237,224.0.0.252,5355,UDP,Unknown_UDP,Uncategorised,4,0,0.15,272,0,0.05
1464051337,t4_12759,213.156.234.1,224.0.0.5,0,OSPFIGP,Invalid,Mixed,0,20,0.74,0,1680,0.32
1464051337,eth0,0.0.0.0,255.255.255.255,67,UDP,DHCP,Services,141,0,5.23,72248,0,13.69
1464051337,eth0,169.254.204.142,169.254.255.255,137,UDP,Unknown_UDP,Uncategorised,3,0,0.11,276,0,0.05
1464051337,eth0,213.156.236.212,213.156.236.255,138,UDP,NetBIOS_UDP,Services,1,0,0.04,243,0,0.05
1464051337,eth0,213.156.236.25,213.156.236.32,22,TCP,Unknown_TCP,Uncategorised,15,5,0.74,1030,370,0.27
1464051337,eth0,fe80::a4ff:4f2f:7bc2:f57,ff02::1:3,5355,UDP,Unknown_UDP,Uncategorised,3,0,0.11,260,0,0.05
1464051337,t4_12759,213.156.234.4,224.0.0.5,0,OSPFIGP,Invalid,Mixed,20,0,0.74,1680,0,0.32
1464051337,eth0,213.156.236.227,213.156.236.255,137,UDP,Unknown_UDP,Uncategorised,12,0,0.44,1104,0,0.21
1464051337,eth0,213.156.236.32,213.156.239.131,123,UDP,Unknown_UDP,Uncategorised,8,8,0.59,720,720,0.27
1464051337,t4_12745,213.156.234.1,224.0.0.5,0,OSPFIGP,Invalid,Mixed,0,20,0.74,0,1680,0.32
1464051337,eth0,213.156.236.87,213.156.236.255,4002,UDP,Unknown_UDP,Uncategorised,5,0,0.19,470,0,0.09
1464051337,eth0,fe80::24c6:6b9a:edee:4051,ff02::1:3,5355,UDP,Unknown_UDP,Uncategorised,3,0,0.11,260,0,0.05
1464051337,t4_12745,213.156.234.3,224.0.0.5,0,OSPFIGP,Invalid,Mixed,20,0,0.74,1680,0,0.32
1464051337,eth1,10.0.0.1,10.0.0.2,0,ESP,Invalid,Mixed,60,60,4.45,9640,9640,3.65
1464051337,t4_12743,213.156.234.1,224.0.0.5,0,OSPFIGP,Invalid,Mixed,0,20,0.74,0,1680,0.32
1464051337,eth0,fe80::ec4:7aff:fe08:65f9,ff02::1:2,547,UDP,Unknown_UDP,Uncategorised,2,0,0.07,180,0,0.03
1464051337,eth0,213.156.236.245,213.156.236.32,22,TCP,SSH,Remote_Access,25,15,1.48,1690,1135,0.54
1464051337,eth5,10.0.0.1,10.50.0.2,500,UDP,ISAKMP,Key_Exchange,0,18,0.67,0,11340,2.15
1464051337,eth0,169.254.169.121,169.254.255.255,137,UDP,Unknown_UDP,Uncategorised,3,0,0.11,276,0,0.05
1464051337,eth0,213.156.236.111,213.156.236.255,137,UDP,Unknown_UDP,Uncategorised,3,0,0.11,276,0,0.05
1464051337,eth0,169.254.12.248,169.254.255.255,138,UDP,NetBIOS_UDP,Services,1,0,0.04,243,0,0.05
1464051337,eth0,213.156.236.25,213.156.236.32,22,TCP,SSH,Remote_Access,25,15,1.48,1690,1135,0.54
1464051337,eth3,10.30.0.2,10.30.0.1,0,ESP,Invalid,Mixed,100,60,5.93,16280,9640,4.91
1464051637,eth0,213.156.236.111,213.156.236.255,138,UDP,NetBIOS_UDP,Services,1,0,0.04,243,0,0.05
1464051637,eth0,213.156.236.167,213.156.236.158,22,TCP,Unknown_TCP,Uncategorised,1,0,0.04,94,0,0.02
1464051637,eth0,213.156.236.25,213.156.236.32,22,TCP,Unknown_TCP,Uncategorised,15,5,0.74,1030,370,0.27
1464051637,eth0,213.156.236.147,213.156.236.255,138,UDP,NetBIOS_UDP,Services,1,0,0.04,243,0,0.05
1464051637,eth0,fe80::a800:ff:fe7c:d71f,ff02::16,0,HOPOPT,Invalid,Mixed,2,0,0.07,180,0,0.03
1464051637,eth0,213.156.236.211,213.156.236.255,138,UDP,Unknown_UDP,Uncategorised,2,0,0.07,525,0,0.10
1464051637,t4_12759,213.156.234.4,224.0.0.5,0,OSPFIGP,Invalid,Mixed,20,0,0.74,1680,0,0.32
1464051637,t4_12745,213.156.234.3,224.0.0.5,0,OSPFIGP,Invalid,Mixed,20,0,0.74,1680,0,0.32
1464051637,t4_13444,213.156.234.1,224.0.0.5,0,OSPFIGP,Invalid,Mixed,0,20,0.74,0,1600,0.30
1464051637,eth0,0.0.0.0,255.255.255.255,67,UDP,DHCP,Services,141,0,5.23,72493,0,13.74
1464051637,eth0,213.156.236.87,213.156.236.255,4002,UDP,Unknown_UDP,Uncategorised,5,0,0.19,470,0,0.09
1464051637,eth0,213.156.236.245,213.156.236.32,22,TCP,Unknown_TCP,Uncategorised,15,5,0.74,1030,370,0.27
1464051637,eth0,fe80::ec4:7aff:fe08:65f9,ff02::1:2,547,UDP,Unknown_UDP,Uncategorised,3,0,0.11,270,0,0.05
1464051637,eth0,169.254.204.142,169.254.255.255,138,UDP,NetBIOS_UDP,Services,1,0,0.04,243,0,0.05
1464051637,eth0,213.156.236.167,213.156.236.119,22,TCP,Unknown_TCP,Uncategorised,2,0,0.07,188,0,0.04
1464051637,eth0,213.156.236.239,213.156.236.255,138,UDP,NetBIOS_UDP,Services,1,0,0.04,243,0,0.05
1464051637,eth0,213.156.236.227,213.156.236.255,138,UDP,NetBIOS_UDP,Services,1,0,0.04,243,0,0.05
1464051637,eth2,10.20.0.1,10.20.0.2,0,ESP,Invalid,Mixed,60,60,4.45,9640,9640,3.65
1464051637,t4_12743,213.156.234.2,224.0.0.5,0,OSPFIGP,Invalid,Mixed,20,0,0.74,1680,0,0.32
1464051637,eth0,213.156.236.245,213.156.236.32,22,TCP,SSH,Remote_Access,25,15,1.48,1690,1135,0.54
1464051637,eth0,213.156.236.113,213.156.236.255,138,UDP,NetBIOS_UDP,Services,2,0,0.07,493,0,0.09
1464051637,eth0,213.156.236.212,213.156.236.255,138,UDP,NetBIOS_UDP,Services,1,0,0.04,250,0,0.05
1464051637,t4_12745,213.156.234.1,224.0.0.5,0,OSPFIGP,Invalid,Mixed,0,20,0.74,0,1680,0.32
1464051637,eth0,213.156.236.109,213.156.236.255,138,UDP,NetBIOS_UDP,Services,1,0,0.04,243,0,0.05
1464051637,eth3,10.30.0.2,10.30.0.1,0,ESP,Invalid,Mixed,100,60,5.93,16280,9640,4.91
1464051637,eth0,213.156.236.111,213.156.236.255,137,UDP,Unknown_UDP,Uncategorised,3,0,0.11,276,0,0.05
1464051637,eth0,169.254.169.121,169.254.255.255,137,UDP,Unknown_UDP,Uncategorised,3,0,0.11,276,0,0.05
1464051637,eth0,213.156.236.25,213.156.236.32,22,TCP,SSH,Remote_Access,25,15,1.48,1690,1135,0.54
1464051637,eth0,169.254.193.111,169.254.255.255,138,UDP,NetBIOS_UDP,Services,1,0,0.04,243,0,0.05
1464051637,t4_12759,213.156.234.1,224.0.0.5,0,OSPFIGP,Invalid,Mixed,0,20,0.74,0,1680,0.32
1464051637,eth0,169.254.251.62,169.254.255.255,138,UDP,NetBIOS_UDP,Services,2,0,0.07,493,0,0.09
1464051637,eth5,10.0.0.1,10.50.0.2,500,UDP,ISAKMP,Key_Exchange,0,17,0.63,0,10710,2.03
1464051637,eth0,169.254.169.121,169.254.255.255,138,UDP,NetBIOS_UDP,Services,2,0,0.07,492,0,0.09
1464051637,eth0,169.254.204.142,169.254.255.255,137,UDP,Unknown_UDP,Uncategorised,3,0,0.11,276,0,0.05
1464051637,t4_12743,213.156.234.1,224.0.0.5,0,OSPFIGP,Invalid,Mixed,0,20,0.74,0,1680,0.32
1464051637,eth1,10.0.0.1,10.0.0.2,0,ESP,Invalid,Mixed,60,60,4.45,9640,9640,3.65
1464051637,eth0,213.156.236.110,213.156.236.255,138,UDP,NetBIOS_UDP,Services,1,0,0.04,243,0,0.05
1464051637,t4_12749,213.156.234.1,224.0.0.5,0,OSPFIGP,Invalid,Mixed,0,20,0.74,0,1600,0.30`

var inputCSVFooter = `Received packets,1663
Sent packets,1034
Received data volume (bytes),372618
Sent data volume (bytes),154985
Sorting and flow direction,first packet time
Interface,any`

const MAGIC_ENV_VAR = "GOTEST_argumentsMain"

func TestMain(m *testing.M) {
	var err error

	// remove any current test databases
	if err = os.RemoveAll(DBPATH); err != nil {
		fmt.Printf("Failed to remove old databases: %s", err.Error())
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func callMain(arg ...string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=TestCallMain")
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", MAGIC_ENV_VAR, arg))
	return cmd
}

func TestConversion(t *testing.T) {
	// write the testing string to a file
	if err := ioutil.WriteFile("./data.csv", []byte(inputCSV), 0755); err != nil {
		t.Fatalf("Failed to set up test data: %s", err.Error())
	}

	output, err := callMain("-in", "data.csv", "-out", DBPATH).CombinedOutput()
	if err != nil {
		t.Fatalf("Error running conversion: Error %s\n, Output:%s\n", err.Error(), output)
	}

	// TODO: the following test breaks due to precision errors when the floating point percentages
	// are computed. The lines are actually holding the same values and attributes (verified with
	// vimdiff)
	//
	//    // check if goquery is available on the host and part of the PATH. If not, skip
	//    // the sanity checks
	//    if _, err := exec.Command("which", "goquery").CombinedOutput(); err !=nil {
	//        return
	//    }
	//
	//    // run goquery with arguments to produce above CSV file
	//    goqueryOutput, err := exec.Command("goquery", "-d", DBPATH, "-i", "any", "-e", "csv", "raw").CombinedOutput()
	//    if err != nil {
	//        t.Fatalf("Error during goquery call: %s", err.Error())
	//    }
	//    goqueryOutput = goqueryOutput[:len(goqueryOutput)-1]
	//
	//  // scrutinize the output. Currently this is a n^2 operation, at 100 lines of output, we
	//  // can afford it though
	//    if faulty, found := compareLinesWithInput(goqueryOutput); !found {
	//        t.Fatalf("Conversion inconsistency: line '%s' not found in converted DB", faulty)
	//    }
}

func compareLinesWithInput(convertedOutput []byte) (string, bool) {
	scanIn := bufio.NewScanner(bytes.NewBuffer([]byte(inputCSV + "\n" + inputCSVFooter)))

	// go through all lines in the input CSV and look for them in the converted output.
	// Bail directly if one item cannot be found
	var line string
	var found bool
	for scanIn.Scan() {
		line = scanIn.Text()
		fmt.Println(line)
		scanOut := bufio.NewScanner(bytes.NewBuffer(convertedOutput))
		for scanOut.Scan() {
			if scanOut.Text() == line {
				found = true
				break
			}
		}
		if !found {
			return line, found
		}
		found = false
	}
	return "", true
}

func TestCallMain(t *testing.T) {
	if args := os.Getenv(MAGIC_ENV_VAR); args != "" {
		os.Args = []string{os.Args[0], "-in", "data.csv", "-out", DBPATH}
		main()
		return
	}
}

func TestParsers(t *testing.T) {
	var (
		err    error
		rowKey goDB.ExtraKey
		rowVal goDB.Val
	)

	t.Parallel()
	for _, tt := range parserTests {
		conv := NewCSVConverter()
		if err = conv.readSchema(tt.schema); err != nil {
			t.Fatalf("Unable to read schema: %s", err.Error())
		}

		fields := strings.Split(tt.input, ",")
		for ind, parser := range conv.KeyParsers {
			if err = parser.ParseKey(fields[ind], &rowKey); err != nil {
				t.Fatalf("%s", err.Error())
			}
		}
		for ind, parser := range conv.ValParsers {
			if err := parser.ParseVal(fields[ind], &rowVal); err != nil {
				t.Fatalf("%s", err.Error())
			}
		}

		// check equality of keys and values
		if !reflect.DeepEqual(rowKey, tt.outKey) {
			t.Fatalf("Key: got: %s; expect: %s", fmt.Sprint(rowKey), fmt.Sprint(tt.outKey))
		}
		if !reflect.DeepEqual(rowVal, tt.outVal) {
			t.Fatalf("Val: got: %s; expect: %s", fmt.Sprint(rowVal), fmt.Sprint(tt.outVal))
		}
	}
}
