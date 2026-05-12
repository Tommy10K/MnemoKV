// Command workload is the synthetic traffic generator. The full implementation
// lands in a later phase; this stub keeps the build green and the binary
// surface stable.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "mnemokv-workload: not implemented yet (planned for phase 13)")
	os.Exit(1)
}
