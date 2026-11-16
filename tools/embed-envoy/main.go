// Copied into the selected Core module so upstream types and version semantics
// are compiled directly. A changed embedding API therefore fails at compile time.
package main

import (
	"crypto/sha256"
	"debug/elf"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pomerium/pomerium/pkg/envoy/envoyversion"
)

func main() {
	if len(os.Args) != 4 {
		panic("usage: embed-envoy BINARY FULL_SOURCE_COMMIT BUILD_TIMESTAMP")
	}
	binary, commit, timestamp := os.Args[1], os.Args[2], os.Args[3]
	f, err := elf.Open(binary)
	if err != nil {
		panic(err)
	}
	section := f.Section(".license")
	if section == nil {
		panic("missing Envoy license stamp")
	}
	license, err := section.Data()
	if err != nil {
		panic(err)
	}
	if strings.TrimRight(string(license), "\x00\n") != "Apache-2.0" {
		panic("Envoy is not stamped Apache-2.0")
	}
	f.Close()
	b, err := os.ReadFile(binary)
	if err != nil {
		panic(err)
	}
	version := envoyversion.Version()
	if version == "" {
		panic("missing upstream Envoy dependency version")
	}
	lock := envoyversion.Lockfile{Filename: "envoy-linux-amd64", Size: int64(len(b)), Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(b)), Version: version, Labels: map[string]string{
		"BUILD_SCM_REMOTE":   "https://github.com/pomerium/envoy-custom",
		"BUILD_SCM_REVISION": commit, "BUILD_TIMESTAMP": timestamp,
		"org.opencontainers.image.source":   "https://github.com/fogandfrog/pomerium-build",
		"org.opencontainers.image.licenses": "Apache-2.0",
	}}
	out, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(binary+".lock", out, 0644); err != nil {
		panic(err)
	}
}
