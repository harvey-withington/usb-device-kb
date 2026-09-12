// validate checks docks.json and devices.json and exits non-zero with
// every problem listed. CI runs it on each push and pull request.
//
//	go run ./cmd/validate [dir]
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/harvey-withington/usb-device-kb/kbdata"
)

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	failed := false
	report := func(name string, problems []string, err error) {
		switch {
		case err != nil:
			fmt.Printf("%s: %v\n", name, err)
			failed = true
		case len(problems) > 0:
			fmt.Printf("%s: %d problem(s)\n", name, len(problems))
			for _, p := range problems {
				fmt.Printf("  - %s\n", p)
			}
			failed = true
		default:
			fmt.Printf("%s: ok\n", name)
		}
	}

	docks, err := kbdata.LoadDocks(filepath.Join(dir, "docks.json"))
	var dockProblems []string
	if err == nil {
		dockProblems = kbdata.ValidateDocks(docks)
	}
	report("docks.json", dockProblems, err)

	devices, err := kbdata.LoadDevices(filepath.Join(dir, "devices.json"))
	var deviceProblems []string
	if err == nil {
		deviceProblems = kbdata.ValidateDevices(devices)
	}
	report("devices.json", deviceProblems, err)

	if failed {
		os.Exit(1)
	}
}
