package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var exampleCmd = &cobra.Command{
	Use:   "examples",
	Short: "Print examples invocations of goquery",
	Run:   printExample,
}

var examples = `
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

      goquery -i eth0,t4_1234 -f "-1d" -c "snet=213.156.236.0/24 AND dnet=213.156.237.0/25" \
        talk_conv

  * Get the top 10 (-n) source-ip/destination-ip triples from all time (-f "-9999d") whose
    source or destination was in 172.27.0.0/16:

      goquery -i eth0 -f "-9999d" -c "snet = 172.27.0.0/16 | dnet = 172.27.0.0/16" -n 10 \
        "sip,dip"
`

func printExample(cmd *cobra.Command, args []string) {
	fmt.Println(examples)
}
