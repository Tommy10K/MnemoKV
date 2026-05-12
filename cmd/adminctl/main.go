// Command adminctl is the cluster admin CLI. The full implementation lands in
// a later phase; this stub keeps the build green and the binary surface stable.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "mnemokv-adminctl: not implemented yet (planned for phase 12+)")
	os.Exit(1)
}
