#!/bin/bash

echo -e "package goDB\n\nvar IPProtocols = map[int] string {"
egrep -v "^#" /etc/protocols | egrep -v "^ip\s+0\s+IP" | egrep -v "^(\s+)?$" | sort -unk2 | awk '{print "  " $2 ": \"" $3 "\","}'
echo -e "  255: \"UNKNOWN\",\n}\n\nfunc GetIPProto(id int) string {\n  return IPProtocols[id]\n}\n"

echo -e "\nvar IPProtocolIDs = map[string] int {"
egrep -v "^#" /etc/protocols | egrep -v "^ip\s+0\s+IP" | egrep -v "^(\s+)?$" | grep -v "for experimentation and testing" | sort -unk2 | awk '{print "  \"" $3 "\": " $2 ","}'
echo -e "  \"UNKNOWN\": 255,\n}\n\nfunc GetIPProtoID(name string) (uint64, bool) {\n  ret, ok := IPProtocolIDs[name]\n  return uint64(ret), ok\n}\n"

exit 0
