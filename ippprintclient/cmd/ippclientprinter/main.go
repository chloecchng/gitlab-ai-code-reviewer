package main

import (
	"flag"

	"bitbucket.org/papercutsoftware/pmitc-coordinator/ippprintclient"
)

func main() {
	flag.Parse()
	ippprintclient.Main()
}
