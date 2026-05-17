// Command cssi-server runs the CSSI storage server: it owns the LVM volume
// group on the host and exports each provisioned LV over NFS.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/michaelcourcyo/cssi/internal/version"
	"github.com/michaelcourcyo/cssi/pkg/server"
)

// main parses the two server flags (port and vg), then runs the gRPC API.
func main() {
	var (
		port        = flag.Int("port", 9000, "TCP port the CSSI gRPC API listens on")
		vgName      = flag.String("vg", "cssi", "LVM Volume Group name to carve volumes from")
		showVersion = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("cssi-server", version.String())
		return
	}

	s := server.New(server.Config{
		Port:   *port,
		VGName: *vgName,
	})

	if err := s.Run(); err != nil {
		log.Printf("server exited: %v", err)
		os.Exit(1)
	}
}
