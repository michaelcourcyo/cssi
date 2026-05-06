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

// main runs the CSSI storage server.
// It parses command-line flags for the API listen address, LVM volume group name, export root directory, and a version flag; if the version flag is set it prints the binary version and exits. It constructs the server from the parsed flags, starts it, and exits with status 1 if the server returns an error.
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
