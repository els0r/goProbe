#!/usr/bin/perl
###############################################################################
#
# gp_status.pl
#
# Written by Lennart Elsen lel@open.ch, December 2015
# Copyright (c) 2015 Open Systems AG, Switzerland
# All Rights Reserved.
#
# Helper script to correctly format goprobe's status output coming from the
# control socket.
#
################################################################################
use strict;
use warnings;

my $MAX_WIDTH = 8;

my $SL_INDENT = 54;

my $SL_NOCOLORS=0;

my $SL_RED    = '';
my $SL_GREEN  = '';
my $SL_YELLOW = '';
my $SL_NORMAL = '';

if(!$ENV{'NOCOLORS'}) {
  $SL_RED   =sprintf("%c[1;31m",27);
  $SL_GREEN =sprintf("%c[1;32m",27);
  $SL_YELLOW=sprintf("%c[1;33m",27);
  $SL_NORMAL=sprintf("%c[0;39m",27);
}

my $_status_open = 0;

sub statusline {
  my $string = shift;
  my $dots = $SL_INDENT - length($string);
  $dots=1 if ($dots<1);
  printf("- %-s%-s",$string,"."x$dots);
  $_status_open = 1;
}

sub statusok {
  my $resultcode = shift;
  my $shortmessage = shift || "";

  if($resultcode =~ /^(ok|success)$/) {
    printf("[  %sOK%s  ] %s\n",$SL_GREEN,$SL_NORMAL,$shortmessage);
  } elsif($resultcode =~ /^(warn|warning)$/) {
    printf("[ %sATTN%s ] %s\n",$SL_YELLOW,$SL_NORMAL,$shortmessage);
  } else {
    printf("[%sFAILED%s] %s\n",$SL_RED,$SL_NORMAL,$shortmessage);
  }

  $_status_open = 0;
}

sub statusline_is_open {
  return $_status_open;
}

sub humanize {
    my $precision = shift; my $div = shift; my $count = 0;

    my %units = (
        1024      =>  ["B", "kB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"],
        1000      =>  ["", "K", "M", "G", "T", "P", "E", "Z", "Y"],
    );

    my @ret;
    foreach my $item ( @_ ){
        $count=0;
        while ($item > $div) {
            $item /= $div;
            $count++;
        }

        my $fmt_item = ( $count == 0 ? $item : sprintf("%${precision}f %s", $item, @{$units{$div}}[$count]) );

        my $spaces = $MAX_WIDTH - length($fmt_item);
        $fmt_item = sprintf("%-s%-s"," "x$spaces, $fmt_item);

        push(@ret, $fmt_item);
    }
    return @ret;
}

my $lnum=0;
my ($detailed, $time_elapsed);
my ($t_rcv_gp, $t_rcv_pcap, $t_drop_pcap, $t_ifdrop);
my $iface_states;

# this script reads from STDIN
while(<>) {
    chomp($_);
    if ($lnum == 0) {
        ($detailed, $time_elapsed) = split(" ", $_);
        $lnum++; next;
    }

    my ($iface, $state, $rcv_gp, $rcv_pcap, $drop_pcap, $ifdrop) = split(" ", $_);
    $iface_states->{$iface} = {
        state => $state,
        rcv_gp => $rcv_gp, rcv_pcap => $rcv_pcap,
        drop_pcap => $drop_pcap, ifdrop => $ifdrop,
    };

    $t_rcv_gp += $rcv_gp;
    $t_rcv_pcap += $rcv_pcap;
    $t_drop_pcap += $drop_pcap;
    $t_ifdrop += $ifdrop;

    $lnum++;
}

my $last_write = "${time_elapsed}s ago";

print "Interface Capture Statistics:\n
       last writeout: ", sprintf("%-s%-s", " "x($MAX_WIDTH-length($last_write)), $last_write),"
    packets received: ", humanize(".2", "1000", $t_rcv_gp),"
     dropped by pcap: ", humanize(".2", "1000", $t_drop_pcap),"
    dropped by iface: ", humanize(".2", "1000", $t_ifdrop),"\n\n";

# print detailed statistics
if ($detailed) {
    # prepare header assumes that SL_INDENT = 54 from statusline
    my $spaces=66;
    my $header="PKTS RCV      DROP   IF DROP";
    $header = sprintf("%-s%-s"," "x$spaces, $header);
    print "$header\n";

    # prepare individual iface stats
    foreach my $iface (sort keys %{ $iface_states } ) {
        statusline("$iface ");
        my $stats = $iface_states->{$iface};
        if ($stats->{'state'} eq "active") {
            statusok("ok", " " .
                join("  ",
                    humanize(".2", "1000",
                        $stats->{rcv_gp},
                        $stats->{drop_pcap},
                        $stats->{ifdrop},
                    ),
                ),
            );
        } else {
            statusok("warn", "not capturing");
        }
    }

    print "\n";
}

1;
