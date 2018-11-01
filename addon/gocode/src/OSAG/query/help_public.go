/////////////////////////////////////////////////////////////////////////////////
//
// help.go
//
// Wrapper for all help printing related functions for goQuery
//
// Written by Lennart Elsen lel@open.ch, April 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

// +build !OSAG

package main

import "fmt"
import "sort"

var helpBase string = `USAGE:

    goquery -i <interfaces> [-hax] [-in|-out|-sum] [-n <max_n>] [-resolve]
    [-e txt|csv|json|influxdb] [-d <db-path>] [-f <timestamp>] [-l <timestamp>]
    [-c <conditions>] [-s <column>] {COLUMNS|QUERY_TYPE}

    Flow database query tool to extract flow statistics from the goDB database
    created by goProbe. By default, output is written to STDOUT, sorted by overall
    (incoming and outgoing) data volume in descending order.

    COLUMNS
        A comma separated list of columns over which to perform the "GROUP BY"/drilldown.
        Available columns:
          sip (or src)   source ip
          dip (or dst)   destination ip
          dport          destination port
          iface          interface
          proto          protocol (e.g. UDP, TCP)
          time           timestamp

    QUERY_TYPE
        Type of query to perform (top talkers or top applications). This allows you to
        conveniently specify commonly used column combinations.
          talk_src        top talkers by source IP (default)
                          (equivalent to columns "sip")
          talk_dst        top talkers by destination IP
                          (equivalent to columns "dip")
          talk_conv       top talkers by IP pairs ("conversation")
                          (equivalent to columns "sip,dip")
          apps_port       top applications by protocol:[port]
                          (equivalent to columns "dport,proto")
          agg_talk_port   aggregation of conversation and applications
                          (equivalent to columns "sip,dip,dport,proto")
          raw             a raw dump of all flows, including timestamps and interfaces
                          (equiv. to columns "time,iface,sip,dip,dport,proto")
`
var examples string = `
EXAMPLES

    * Show the top 5 (-n) IP pairs (talk_conv) over the default data presentation period
      (30 days) on a specific interface (-i):

        goquery -i eth0 -n 5 talk_conv

      equivalently you could also write

        goquery -i eth0 -n 5 sip,dip

    * Show the top 10 (-n) dport-protocol pairs over collected data from two days ago (-f, -l)
      ordered by the number of packets (-s packets) in ascending order (-a):

        goquery -i eth0 -n 10 -s packets -a -f "-2d" -l "-1d" apps_port

    * Get the total traffic volume over eth0 and t4_1234 in the last 24 hours (-f)
      between source network 213.156.236.0/24 (-c snet) and destination network
      213.156.237.0/25 (-c dnet):

        goquery -i eth0,t4_1234 -f "-1d" -c "snet=213.156.236.0/24 AND dnet=213.156.237.0/25" talk_conv

    * Get the top 10 (-n) layer-7-protocol/source-ip/destination-ip triples from all time
      (-f "-9999d") whose source or destination was in 172.27.0.0/16:

        goquery -i eth0 -f "-9999d" -c "snet = 172.27.0.0/16 | dnet = 172.27.0.0/16" -n 10 "sip,dip"
`
var admin string = `
    Advanced maintenance options (should not be used in interactive mode):

    -clean <timestamp>
        Remove all database rows before given timestamp (retention time).
        Handle with utmost care, all changes are permanent and cannot be undone!
        Allowed formats are identical to -f/-l parameters.

    -wipe
        Wipe all database entries from disk.
        Handle with utmost care, all changes are permanent and cannot be undone!
`

var helpMap map[string]string = map[string]string{
	"i": `
    -i <interfaces>

        Interfaces for which the query should be performed (e.g. "eth0", "eth0,t4_33760")
        You can specify "ANY" to query all interfaces.
`,
	"h": `
    -h, --help

        Display this help text.
`,
	"help-admin": `
    -help-admin

        Display advanced options for database maintenance.
`,
	"f": `
   -f / -l "<date>"

        Lower / upper bound on flow timestamp. Allowed formats are:
          1357800683                            EPOCH
          Mon Jan _2 15:04:05 2006              ANSIC
          Mon Jan 02 15:04:05 -0700 2006        RUBY DATE
          02 Jan 06 15:04 -0700                 RFC822 with numeric zone
          2006-01-02T15:04:05Z07:00             RFC3339
          02 Jan 06 15:04 -0700                 RFC822 with numeric zone
          Mon, 02 Jan 2006 15:04:05 -0700       RFC1123 with numeric zone

          02.01.2006 15:04:05                   CUSTOM
          02.01.2006 15:04
          02.01.06 15:04
          2006-01-02 15:04:05
          2006-01-02 15:04
          2.1.06 15:04:05
          2.1.06 15:04
          2.1.2006 15:04:05
          2.1.2006 15:04
          02.1.2006 15:04:05
          02.1.2006 15:04
          2.01.2006 15:04:05
          2.01.2006 15:04
          02.1.06 15:04:05
          02.1.06 15:04
          2.01.06 15:04:05
          2.01.06 15:04

          -15d:04h:05m                          RELATIVE

        Relative time will be evaluated with respect to NOW. The call can
        be varied to include any (integer) combination of days (d), hours
        (h) and minutes (m), e.g.

          -15d:04h:05m, -15d:5m, -15d, -5m, -4h, -4h:05m, etc.

        NOTE: there is no attribute for "month" as it collides with "m"
              used for minutes. If you plan to run queries over a time
              span of several months, simply specify the number of days
              that should be taken into account (e.g. "-45d").

        TIME ZONES:
              all CUSTOM time formats support an offset from UTC. It can be
              used to evaluate dates in timezones different from the one used
              on the host (e.g. Europe/Zurich - CEST). The format is {+,-}0000.
              For a host in San Fransisco (PDT), a difference of -7 hours to
              UTC is given. The date would be passed as

                02.01.06 -0700

              In Sydney time (AEST), the same date would be passed as

                02.01.06 +1000

              while in Tehran (IRDT) it would be written as

                02.01.06 +0430
`,
	"c": `
    -c "<conditional>"

        The conditional consists of multiple conditions chained together
        via logical operators. The condition precedence is set via bracing of
        individual condition chains.

        A single condition consists of an attribute, a comparative operator,
        and a value against which the attribute is checked, e.g.:

            dport <= 1024

        ATTRIBUTES:

          Talker:
            dip (or dst)       Destination IP/Hostname
            sip (or src)       Source IP/Hostname
            host               Source IP/Hostname or Destination IP/Hostname

            EXAMPLE: "dip != 192.168.1.34 & sip = 172.16.22.15" is equivalent to
                     "src != 192.168.1.34 & dst = 172.16.22.15"
                     "host = 192.168.1.34" is equivalent to
                     "(sip = 192.168.1.34 | dip = 192.168.1.34)"
                     "host != 192.168.1.34" is equivalent to
                     "(sip != 192.168.1.34 & dip != 192.168.1.34)"
                     "sip = foo.com" is equivalent to
                     "sip = 2a00:50::1009 | sip = 173.194.116.40"
                     (assuming that those are the A and AAAA records of foo.com)

          Talker by network:
            dnet        Destination network in CIDR notation
            snet        Source network in CIDR notation
            net         Source network or destination network

            EXAMPLE: "dnet = 192.168.1.0/25 | snet = 172.16.22.0/12"
                     "net = 192.168.1.0/24" is equivalent to
                     "(snet = 192.168.1.0/24 | dnet = 192.168.1.0/24)"
                     "net != 192.168.1.0/24" is equivalent to
                     "(snet != 192.168.1.0/24 & dnet != 192.168.1.0/24)"

          Application:
            dport       Destination port
            proto       IP protocol

            EXAMPLE: "dport = 22 & proto = TCP"

        COMPARATIVE OPERATORS:

          Base    Description            Other representations

             =    equal to               eq, -eq, equals, ==, ===
            !=    not equal to           neq, -neq, ne, -ne
            <=    less or equal to       le, -le, leq, -leq
            >=    greater or equal to    ge, -ge, geq, -geq
             <    less than              less, l, -l, lt, -lt
             >    greater than           greater, g, -g, gt, -gt

        All of the items under "Other representations" (except for "===" and "==")
        must be enclosed by whitespace.

          NOTE: In case the attribute involves an IP address, only "=" and "!="
                are supported.

        Individual conditions can be chained together via logical operators, e.g.

            ! dport = 8080 | dport = 443 & proto = TCP

        LOGICAL OPERATORS:

          Base    Description            Other representations
             !    unary negation         not
             &    and                    and, &&, *
             |    or                     or, ||, +

        The representations "not", and", and "or" require enclosing whitespace.

        PRECEDENCE:

        In terms of logical operator precendence, NOT is evaluated before AND and
        AND is evaluated before OR.

        Thus above expression would be evaluated as

            (! dport = 8080) | ( dport = 443 & proto = TCP)

        Precedence can be enforced by bracing condition chains appropriately, e.g.

            ! (( dport = 8080 | dport = 443 ) & proto = TCP )

        NOT simply negates whatever comes after it. For example

            (! dport = 8080) | (! (dport = 443 & proto = TCP))

        is equivalent to

            dport != 8080 | (dport != 443 | proto != TCP)).

        The braces "[]" and "{}" can also be used.

        SYNTAX

        The condition can be expressed in different syntaxes, which can be combined
        arbitrarily to the user's liking. Consider the following conditional:

            ( proto = TCP & snet != 192.168.0.0/16 ) & ( dport <= 1024 | dport >= 443 )

        It can also be provided as:

        ( proto eq  TCP and snet neq 1.2.0.0/16 ) and ( dport   le 1024 or dport   ge 443 )
        [ proto  =  TCP   * snet  != 1.2.0.0/16 ]   * [ dport   <= 1024  + dport   >= 443 ]
        { proto -eq TCP  && snet -ne 1.2.0.0/16 }   * { dport -leq 1024 || dport -geq 443 }

        and any other combination of the allowed representations.
`,
	"d": `
    -d <db-path>

        Path to goDB database directory <db-path>. By default,
        the database path from the configuration file is used.
        If it does not exist, an error will be thrown.

        This also implies that you have to explicitly specify
        the path if you analyze data on a different host without
        goProbe.
`,
	"e": `
    -e <format>

        Output format:
          txt           Output in plain text format (default)
          json          Output in JSON format
          csv           Output in comma-separated table format
`,
	"n": `
    -n <number of entries>

        Maximum number of final entries to show. Defaults to 95% of the overall
        data volume / number of packets (depending on the '-s' parameter). Ignored
        for queries including the "time" field.
`,
	"s": `
    -s <column>

        Sort results by given column name:
          bytes         Sort by accumulated data volume (default)
          packets       Sort by accumulated packets
          time          Sort by time. Forced for queries including the "time" field
`,
	"a": `
    -a
        Sort results in ascending instead of descending order. Forced for queries
        including the "time" field.
`,
	"list": `
    -list, --list
        List all interfaces on which data was captured and written to the database.
`,
	"in": `
    -in
        Take into account incoming data (received packets/bytes). Can be combined
        with -out.
`,
	"out": `
    -out
        Take into account outgoing data (sent packets/bytes). Can be combined
        with -in.
`,
	"sum": `
    -sum
        Sum incoming and outgoing data.
`,
	"x": `
    -x
        Mode for external calls, e.g. from portal. Reduces verbosity of error
        messages to customer friendly text and writes full error messages
        to message log instead.
`,
	"resolve": `
    -resolve
        Resolve top IPs in output using reverse DNS lookups. Off by default.
        If the reverse DNS lookup for an IP fails, the IP is shown instead.
        The lookup is performed for the first '-resolve-rows' (default: 25) rows
        of output.
        Beware: The lookup is carried out at query time; DNS data may have been
        different when the packets were captured.
`,
	"resolve-timeout": `
    -resolve-timeout
        Timeout for (reverse) DNS lookups. Default: 1s
`,
	"resolve-rows": `
    -resolve-rows
        Maximum number of output rows to perform DNS resolution against. Before setting
        this to some high value (e.g. 1000), consider that this may incur a high load on
        the DNS resolver and network! Default: 25
`}

func PrintFlagGenerator(external bool) func(affectedFlag string) {
	var helpString string = helpBase
	if !external {
		return func(affectedFlag string) {
			if affectedFlag == "" {
				fmt.Println(helpString)
				return
			} else if affectedFlag == "admin" {
				fmt.Println(helpString + admin)
				return
			}
			helpString = helpString + helpMap[affectedFlag]
			fmt.Println(helpString)
		}
	}

	// do nothing if external call
	return func(affectedFlag string) {
		return
	}
}

// Returns a function that prints the usage information if external is false and a
// function that prints nothing if external is true.
// Note that the usage information doesn't contain the admin help.
func PrintUsageGenerator(external bool) func() {
	var helpString string = helpBase
	if !external {
		// sort the map alphabetically
		sorted := make([]string, len(helpMap))
		i := 0
		for k, _ := range helpMap {
			sorted[i] = k
			i++
		}
		sort.Strings(sorted)

		return func() {
			for _, mapKey := range sorted {
				helpString = helpString + helpMap[mapKey]
			}
			helpString = helpString + examples
			fmt.Println(helpString)
		}
	}

	// do nothing if external call
	return func() {
		return
	}
}
