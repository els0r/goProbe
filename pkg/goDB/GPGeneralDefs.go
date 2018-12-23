/////////////////////////////////////////////////////////////////////////////////
//
// GPGeneralDefs.go
//
// Type definitions and helper functions used throughout this package
//
// Written by Lennart Elsen      lel@open.ch and
//            Lorenz Breidenbach lob@open.ch, October 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

type DBData struct {
	// counters
	Bytes_rcvd []byte
	Bytes_sent []byte
	Pkts_rcvd  []byte
	Pkts_sent  []byte

	// attributes
	Dip   []byte
	Sip   []byte
	Dport []byte
	Proto []byte

	// metadata (important for folder naming)
	Tstamp int64
	Iface  string
}

// constructor for the DBData struct in case it needs to be set from an external
// go program that included goProbe
func NewDBData(br []byte, bs []byte, pr []byte, ps []byte, dip []byte, sip []byte, dport []byte, proto []byte, tstamp int64, iface string) DBData {
	return DBData{br, bs, pr, ps, dip, sip, dport, proto, tstamp, iface}
}

// GOOGLE's utility functions for printing IPv4/6 addresses ----------------------
// Convert i to hexadecimal string
func itox(i uint, min int) string {

	// Assemble hexadecimal in reverse order.
	var b [32]byte
	bp := len(b)
	for ; i > 0 || min > 0; i /= 16 {
		bp--
		b[bp] = "0123456789abcdef"[byte(i%16)]
		min--
	}

	return string(b[bp:])
}

// Convert i to decimal string.
func itod(i uint) string {
	if i == 0 {
		return "0"
	}

	// Assemble decimal in reverse order.
	var b [32]byte
	bp := len(b)
	for ; i > 0; i /= 10 {
		bp--
		b[bp] = byte(i%10) + '0'
	}

	return string(b[bp:])
}

/// END GOOGLE ///

// convert the ip byte arrays to string. The formatting logic for IPv6
// is directly copied over from the go IP package in order to save an
// additional import just for string operations
func rawIpToString(ip []byte) string {
	var (
		numZeros uint8 = 0
		iplen    int   = len(ip)
	)

	// count zeros in order to determine whether the address
	// is IPv4 or IPv6
	for i := 4; i < iplen; i++ {
		if (ip[i] & 0xFF) == 0x00 {
			numZeros++
		}
	}

	// construct ipv4 string
	if numZeros == 12 {
		return itod(uint(ip[0])) + "." +
			itod(uint(ip[1])) + "." +
			itod(uint(ip[2])) + "." +
			itod(uint(ip[3]))
	} else {
		/// START OF GOOGLE CODE SNIPPET ///
		p := ip

		// Find longest run of zeros.
		e0 := -1
		e1 := -1
		for i := 0; i < iplen; i += 2 {
			j := i
			for j < iplen && p[j] == 0 && p[j+1] == 0 {
				j += 2
			}
			if j > i && j-i > e1-e0 {
				e0 = i
				e1 = j
			}
		}

		// The symbol "::" MUST NOT be used to shorten just one 16 bit 0 field.
		if e1-e0 <= 2 {
			e0 = -1
			e1 = -1
		}

		// Print with possible :: in place of run of zeros
		var s string
		for i := 0; i < iplen; i += 2 {
			if i == e0 {
				s += "::"
				i = e1
				if i >= iplen {
					break
				}
			} else if i > 0 {
				s += ":"
			}
			s += itox((uint(p[i])<<8)|uint(p[i+1]), 1)

		}
		return s
	}
}
