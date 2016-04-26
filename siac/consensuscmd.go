package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/NebulousLabs/Sia/api"
)

var (
	consensusCmd = &cobra.Command{
		Use:   "consensus",
		Short: "Print the current state of consensus",
		Long:  "Print the current state of consensus such as current block, block height, and target.",
		Run:   wrap(consensuscmd),
	}
)

// consensuscmd is the handler for the command `siac consensus`.
// Prints the current state of consensus.
func consensuscmd() {
	var cg api.ConsensusGET
	err := getAPI("/consensus", &cg)
	if err != nil {
		die("Could not get current consensus state:", err)
	}
	fmt.Printf(`Synced: %v
Block:  %v
Height: %v
Target: %v
`, yesNo(cg.Synced), cg.CurrentBlock, cg.Height, cg.Target)
}
