goDB Database Format
====================

Directory Layout
----------------

A goDB is a directory.
The directory contains:
 * a `summary.json` file that provides a brief summary of the contents of the database
 * directories for each network interface for which we have data. The directories are named like the interfaces.

Each of the network interface directories contains:
 * A directory for each day (24-hour period) for which we have data. Each such directory's name is the unix epoch of the first second of its day.

Each of the daily directories contains:
 * One file for each flow attribute we store, i.e. the files `bytes_rcvd.gpf`, `dip.gpf`, `l7proto.gpf`, `pkts_sent.gpf`, `sip.gpf`, `bytes_sent.gpf`, `dport.gpf`, `pkts_rcvd.gpf`, and `proto.gpf`. The gpf file format is documented below.
 * A `meta.json` file containing metadata such as pcap statistics. Its format is documented below.

Example:

    $ tree /path/to/goDB
    /path/to/goDB
    |-- summary.json
    |-- eth0
    |   |-- 1450656000
    |   |   |-- bytes_rcvd.gpf
    |   |   |-- bytes_sent.gpf
    |   |   |-- dip.gpf
    |   |   |-- dport.gpf
    |   |   |-- meta.json
    |   |   |-- l7proto.gpf
    |   |   |-- pkts_rcvd.gpf
    |   |   |-- pkts_sent.gpf
    |   |   |-- proto.gpf
    |   |   `-- sip.gpf
    |   `-- 1450742400
    |       |-- bytes_rcvd.gpf
    |       |-- bytes_sent.gpf
    |       |-- dip.gpf
    |       |-- dport.gpf
    |       |-- meta.json
    |       |-- l7proto.gpf
    |       |-- pkts_rcvd.gpf
    |       |-- pkts_sent.gpf
    |       |-- proto.gpf
    |       `-- sip.gpf
    `-- eth1
        `-- 1452038400
            |-- bytes_rcvd.gpf
            |-- bytes_sent.gpf
            |-- dip.gpf
            |-- dport.gpf
            |-- meta.json
            |-- l7proto.gpf
            |-- pkts_rcvd.gpf
            |-- pkts_sent.gpf
            |-- proto.gpf
            `-- sip.gpf

gpf File Format
---------------

### Structure

Each gpf file consists of a header and up to 512 (LZ4-compressed) blocks.

The *header* consists of 3 sections. Each *section* consists of exactly 512 64-bit values.
 1. The *next_block* section contains for each block in the file the starting position of the block that follows it.
 2. The *timestamp* section contains for each block in the file the timestamp associated with it.
 3. The *length* section contains for each block in the file the uncompressed length of the block.

An uncompressed *block* has the following format:

    64bit epoch timestamp of the block (big-endian)
    value 1
    value 2
    value 3
    ...
    value n
    64bit epoch timestamp of the block (big-endian)

Note that each value is assumed to occupy the same number of bytes.
For example, if we were to store 613 IP addresses in a block, the block's uncompressed size would be 8 + 613 * 16 + 8 = 78'472 bytes.
(8 bytes for the first timestamp, 613 times 16 bytes for each IP, and finally 8 bytes for the closing timestamp)

### Values Stored
We store 9 different gpf files/columns containing different types of values:
* IP addresses (`sip.gpf`, `dip.gpf`) are encoded as 16-byte values. For IPv4 addresses, the last 12 bytes are set to zero.
* Counters (`bytes_sent.gpf`, `bytes_rcvd.gpf`, `pkts_sent.gpf`, `pkts_rcvd.gpf`) are stored as unsigned 64bit big-endian integers.
* Ports (`dport.gpf`) are stored as unsigned 16bit big-endian integers.
* Layer-7-protocol identifiers (`l7proto.gpf`) are stored as unsigned 16bit big-endian integers.
(The identifiers come from libprotoident.)
* Protocol identifiers (`proto.gpf`) are stored as single bytes. (The identifiers are assigned by IANA: http://www.iana.org/assignments/protocol-numbers/protocol-numbers.xhtml)

meta.json Format
----------------

Each `meta.json` file contains a single JSON *Object*.
The object looks like this:

    {
       "blocks" : [
          {
             "flowcount" : 25,
             "traffic" : 245415,
             "timestamp" : 1454512966,
             "packets_logged" : 1036,
             "pcap_packets_received" : 1051,
             "pcap_packets_dropped" : 0,
             "pcap_packets_if_dropped" : 0
          },
          {
             "flowcount" : 30,
             "traffic" : 297709,
             "timestamp" : 1454513266,
             "packets_logged" : 1528,
             "pcap_packets_received" : -1,
             "pcap_packets_dropped" : -1,
             "pcap_packets_if_dropped" : -1
          }
       ]
    }

It has a `blocks` field that contains a list of objects describing each block written for the given day and interface. Each of these objects has a number of fields:
* `flowcount` counts the number of flows stored
* `traffic` counts the total number of bytes of all packets that were captured for the block
* `timestamp` contains the epoch time at which capturing for the block stopped
* `packets_logged` counts the number of packets that were logged by goProbe for the block
* `pcap_packets_received`, `pcap_packets_dropped`, `pcap_packets_if_dropped` are the pcap statistics for the given block.
  Consult http://www.tcpdump.org/manpages/pcap_stats.3pcap.txt for details about their meaning.
  In some cases, the pcap statistics may not have been available when the block was written: All three fields are set to `-1`.


summary.json Format
-------------------

A database contains a single `summary.json` file.
It contains a single JSON *Object*. The object looks like this:

    {
       "interfaces" : {
          "eth0" : {
             "begin" : 1454512966,
             "end" : 1454513866,
             "flowcount" : 47,
             "traffic" : 473562
          },
          "eth1" : {
             "begin" : 1454512966,
             "end" : 1454513866,
             "flowcount" : 0,
             "traffic" : 0
          }
       }
    }

It has a `interfaces` field that contains a list of objects summarising the data captured for each interfaces (for all time, not just a particular day). Each of these objects has a number of fields:
* `begin` contains the epoch time of the earliest block of the interface
* `end` contains the epoch time of the latest block of the interface
* `flowcount` counts the number of flows stored for the interface
* `traffic` counts the total number of bytes of all packets that were captured on the interface

Note: Sice the `summary.json` file may be accessed by multiple processes at the same time, synchronization is necessary.
We create a file `summary.lock` (with flags `O_EXCL|O_CREAT`) in the same directory as the `summary.json`
to indicate that the `summary.json` file is being read/modified. To release the lock, we simply delete `summary.lock`.