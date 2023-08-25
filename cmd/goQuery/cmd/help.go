package cmd

// TODO: This part is currently unused - cross check if that is intentional (in which case it can be removed)
// var usageBase = fmt.Sprintf(`goQuery -i <interfaces> [-h] [-c <conditions>] [-s <column>] [-a] [--in|--out|--sum] [-n <max_n>]
//   [-f|--%s <timestamp>] [-l|--%s <timestamp>] [-e|--%s txt|csv|json] %s
//   [-d|--%s <db-path>] [-q|--%s] [--%s] [--%s] [--%s]
//   [-r|--%s] [--%s] [--%s]
//   [--%s]
// `,
// 	conf.First, conf.Last, conf.ResultsFormat, supportedCmds,
// 	conf.QueryDBPath,
// 	conf.QueryHostsResolution,
// 	conf.QueryServerAddr,
// 	conf.QueryTimeout,
// 	conf.StoredQuery,
// 	conf.DNSResolutionEnabled,
// 	conf.DNSResolutionTimeout,
// 	conf.DNSResolutionMaxRows,
// 	conf.LogLevel,
// )

const helpBase = `
  Flow database query tool to extract flow statistics from the goDB database
  created by goProbe. By default, output is written to STDOUT, sorted by overall
  (incoming and outgoing) data volume in descending order.
`

const helpBaseLong = helpBase + `
  COLUMNS

    A comma separated list of columns over which to perform the "GROUP BY"/drilldown.
    Available columns:

      sip   (or src)   source ip
      dip   (or dst)   destination ip
      dport (or port)  destination port
      proto            protocol (e.g. UDP, TCP)

    Labels which can also be printed as columns:

      hostid           unique ID of the host
      hostname         hostname
      iface            interface
      time             timestamp

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
                      (equivalent to columns "time,iface,sip,dip,dport,proto")
`

var helpMap = map[string]string{
	"Ifaces": `Interfaces for which the query should be performed
(e.g. "eth0 "eth0,t4_33760").
You can specify "ANY" to query all interfaces.
`,
	"Help": `Display this help text.
`,
	"Help-admin": `Display advanced options for database maintenance.
`,
	"First": `Upper/lower bound on flow timestamp

DEFAULTS

  --first will default to the last 30 days if not provided. In case
  a "time" attribute is involved (e.g. for "time" or "raw" queries),
  the default is lowered to the last 24 hours. This is to protect
  against accidentally querying the entire database.

  --last will default to the current time if not provided

ALLOWED FORMATS

  1357800683                            EPOCH
  Mon Jan _2 15:04:05 2006              ANSIC
  Mon Jan 02 15:04:05 -0700 2006        RUBY DATE
  02 Jan 06 15:04 -0700                 RFC822 with numeric zone
  2006-01-02T15:04:05Z07:00             RFC3339
  02 Jan 06 15:04 -0700                 RFC822 with numeric zone
  Mon, 02 Jan 2006 15:04:05 -0700       RFC1123 with numeric zone

  02.01.2006 15:04:05                   CUSTOM
	2006-01-02 15:04:05 -0700
	2006-01-02 15:04 -0700
	2006-01-02 15:04
	06-01-02 15:04:05 -0700
	06-01-02 15:04 -0700
	06-01-02 15:04:05
	06-01-02 15:04
	02-01-2006 15:04:05 -0700
	02-01-2006 15:04 -0700
	02-01-2006 15:04:05
	02-01-2006 15:04
	02-01-06 15:04:05 -0700
	02-01-06 15:04 -0700
	02-01-06 15:04:05
	02-01-06 15:04
	02.01.2006 15:04
	02.01.2006 15:04 -0700
	02.01.06 15:04
	02.01.06 15:04 -0700
	2.1.06 15:04:05
	2.1.06 15:04:05 -0700
	2.1.06 15:04
	2.1.06 15:04 -0700
	2.1.2006 15:04:05
	2.1.2006 15:04:05 -0700
	2.1.2006 15:04
	2.1.2006 15:04 -0700
	02.1.2006 15:04:05
	02.1.2006 15:04:05 -0700
	02.1.2006 15:04
	02.1.2006 15:04 -0700
	2.01.2006 15:04:05
	2.01.2006 15:04:05 -0700
	2.01.2006 15:04
	2.01.2006 15:04 -0700
	02.1.06 15:04:05
	02.1.06 15:04:05 -0700
	02.1.06 15:04
	02.1.06 15:04 -0700
	2.01.06 15:04:05
	2.01.06 15:04:05 -0700
	2.01.06 15:04
	2.01.06 15:04 -0700

  -15d:04h:05m                          RELATIVE
  -15d4h5m

Relative time will be evaluated with respect to NOW. The call can
be varied to include any (integer) combination of days (d), hours
(h) and minutes (m), e.g.

  -15d:04h:05m, -15d:5m, -15d, -5m, -4h, -4h:05m, etc.

NOTE: there is no attribute for "month" as it collides with "m"
      used for minutes. If you plan to run queries over a time
      span of several months, simply specify the number of days
      that should be taken into account (e.g. "-45d").

TIME ZONES:

      All CUSTOM time formats support an offset from UTC. It can be
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
	"Condition": `The conditional consists of multiple conditions chained together
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

    dport (or port) Destination port
    proto           IP protocol

    EXAMPLE: "dport = 22 & proto = TCP" is equivalent to
             "port = 22 & proto = 6"

COMPARATIVE OPERATORS:

  Base    Description            Other representations

     =    equal to               eq, -eq, equals, ==, ===
    !=    not equal to           neq, -neq, ne, -ne
    <=    less or equal to       le, -le, leq, -leq
    >=    greater or equal to    ge, -ge, geq, -geq
     <    less than              less, l, -l, lt, -lt
     >    greater than           greater, g, -g, gt, -gt

All of the items under "Other representations" (except for "===" and
"==") must be enclosed by whitespace.

  NOTE: In case the attribute involves an IP address, only "=" and "!="
        are supported.

Individual conditions can be chained together via logical operators,
e.g.

    ! dport = 8080 | dport = 443 & proto = TCP

LOGICAL OPERATORS:

  Base    Description            Other representations

     !    unary negation         not
     &    and                    and, &&, *
     |    or                     or, ||, +

The representations "not", and", and "or" require enclosing whitespace.

PRECEDENCE:

In terms of logical operator precendence, NOT is evaluated before AND
and AND is evaluated before OR.

Thus above expression would be evaluated as

    (! dport = 8080) | ( dport = 443 & proto = TCP)

Precedence can be enforced by bracing condition chains appropriately,
e.g.

    ! (( dport = 8080 | dport = 443 ) & proto = TCP )

NOT simply negates whatever comes after it. For example

    (! dport = 8080) | (! (dport = 443 & proto = TCP))

is equivalent to

    dport != 8080 | (dport != 443 | proto != TCP)).

The braces "[]" and "{}" can also be used.

SYNTAX

The condition can be expressed in different syntaxes, which can be
combined arbitrarily to the user's liking. Consider the following
conditional:

    ( proto = TCP & snet != 192.168.0.0/16 )
  & ( dport <= 1024 | dport >= 443 )

It can also be provided as:

    ( proto eq  TCP and snet neq 1.2.0.0/16 )
and ( dport   le 1024 or dport   ge 443 )

    [ proto  =  TCP   * snet  != 1.2.0.0/16 ]
  * [ dport   <= 1024  + dport   >= 443 ]

    { proto -eq TCP  && snet -ne 1.2.0.0/16 }
  * { dport -leq 1024 || dport -geq 443 }

and any other combination of the allowed representations.
`,

	"List": `List all interfaces on which data was captured and written
to the database.
`,
	"In": `Take into account incoming data (received packets/bytes). Can be combined
with --out.
`,
	"Out": `Take into account outgoing data (sent packets/bytes). Can be combined
with --in.
`,
	"Sum": `Sum incoming and outgoing data.
`,
}
