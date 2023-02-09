#!/bin/bash

if [[ $( uname -a | grep -i darwin ) ]]; then
	echo -e "// +build darwin\n"
elif [[ $( uname -a | grep -i linux ) ]]; then
	echo -e "// +build linux\n"
fi

echo -e "package protocols"
echo -e ""
echo -e 'import "strings"'
echo -e ""
echo -e "var IPProtocols = map[int] string {"
egrep -v "^#" /etc/protocols | egrep -v "^ip\s+0\s+IP" | egrep -v "^(\s+)?$" | sort -unk2 | awk '{print "  " $2 ": \"" $3 "\","}'
echo -e "  255: \"UNKNOWN\","
echo -e "}"
echo -e ""
echo -e ""
echo -e "func GetIPProto(id int) string {"
echo -e "    return IPProtocols[id]"
echo -e "  }"
echo -e ""
echo -e ""
echo -e "var IPProtocolIDs = map[string] int {"
egrep -v "^#" /etc/protocols | egrep -v "^ip\s+0\s+IP" | egrep -v "^(\s+)?$" | grep -v "for experimentation and testing" | sort -unk2 | awk '{print "  \"" tolower($3) "\": " $2 ","}'
echo -e "  \"unknown\": 255,"
echo -e "}"
echo -e "func GetIPProtoID(name string) (uint64, bool) {"
echo -e "  name = strings.ToLower(strings.TrimSpace(name))"
echo -e "  ret, ok := IPProtocolIDs[name]"
echo -e "  return uint64(ret), ok"
echo -e "}"
echo -e ""

exit 0
