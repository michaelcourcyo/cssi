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

// main parses command-line flags, optionally prints the binary version, and starts the CSSI API server.
// It constructs a server.Config from the flags (listen address, LVM volume group name, and export root),
// runs the server, and logs and exits with status 1 if the server returns an error.
func main() {
	var (
		listen      = flag.String("listen", ":9000", "Address to listen on for the CSSI API")
		vgName      = flag.String("vg", "cssi", "LVM Volume Group name")
		exportRoot  = flag.String("export-root", "/srv/cssi", "Directory under which LV mounts are exported")
		showVersion = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("cssi-server", version.String())
		return
	}

	s := server.New(server.Config{
		ListenAddr: *listen,
		VGName:     *vgName,
		ExportRoot: *exportRoot,
	})

	if err := s.Run(); err != nil {
		log.Printf("server exited: %v", err)
		os.Exit(1)
	}
}
