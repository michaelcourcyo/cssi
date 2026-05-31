// Command cssi-driver runs the CSSI CSI driver (controller + node).
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/michaelcourcyo/cssi/internal/version"
	"github.com/michaelcourcyo/cssi/pkg/driver"
)

// main parses command-line flags and runs the driver. The CSSI server
// endpoint is not a flag: it is read from StorageClass parameters per
// volume.
func main() {
	var (
		endpoint    = flag.String("endpoint", "unix:///csi/csi.sock", "CSI gRPC endpoint")
		nodeID      = flag.String("node-id", "", "Node identifier (usually the K8s node name)")
		showVersion = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("cssi-driver", version.String())
		return
	}

	if *nodeID == "" {
		log.Fatal("--node-id is required")
	}

	d := driver.New(driver.Config{
		Endpoint: *endpoint,
		NodeID:   *nodeID,
	})

	if err := d.Run(); err != nil {
		log.Printf("driver exited: %v", err)
		os.Exit(1)
	}
}
