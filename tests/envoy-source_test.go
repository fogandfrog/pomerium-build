package files

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/pomerium/pomerium/pkg/envoy/envoyversion"
)

func TestIndependentEnvoySourceBuild(t *testing.T) {
	b := Binary()
	lock := Lockfile()
	if lock.Size != int64(len(b)) || lock.Digest != fmt.Sprintf("sha256:%x", sha256.Sum256(b)) {
		t.Fatal("embedded Envoy size or checksum mismatch")
	}
	if lock.Version == "" || lock.Version != envoyversion.Version() {
		t.Fatal("embedded Envoy does not match Core's selected dependency")
	}
	if !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(lock.GitCommit()) || lock.SourceRepo() != "github.com/pomerium/envoy-custom" {
		t.Fatal("missing exact source provenance")
	}
	f, err := elf.NewFile(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s := f.Section(".license")
	if s == nil {
		t.Fatal("missing Envoy license stamp")
	}
	license, err := s.Data()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimRight(string(license), "\x00\n") != "Apache-2.0" {
		t.Fatalf("unexpected Envoy license: %q", license)
	}
}
