package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestModuleSelection(t *testing.T) {
	const hash = "e7b75c3a606de275146897f0f787fe3cf80ead22"
	for _, version := range []string{"v1.2.3", "v1.37.0-rc3.0.20261102191522-e7b75c3a606d"} {
		d := Module{Path: modulePath, Version: version, Sum: "h1:abc", GoModSum: "h1:def"}
		if err := json.Unmarshal([]byte(`{"VCS":"git","URL":"https://github.com/pomerium/envoy-custom","Hash":"`+hash+`"}`), &d.Origin); err != nil {
			t.Fatal(err)
		}
		for _, m := range []Module{{Path: modulePath, Version: version}, {Path: modulePath, Version: "v0.0.1", Replace: &Module{Path: modulePath, Version: version}}} {
			p, err := plan(m, d)
			if err != nil || p.Commit != hash || p.Version != version {
				t.Fatalf("%+v %v", p, err)
			}
		}
		for _, m := range []Module{{}, {Path: modulePath, Version: version, Replace: &Module{Path: "../local"}}, {Path: modulePath, Version: version, Replace: &Module{Path: "github.com/other/fork", Version: version}}, {Path: modulePath, Version: "v0.0.1"}, {Path: modulePath, Version: version, Sum: "h1:wrong"}} {
			if _, err := plan(m, d); err == nil {
				t.Fatalf("accepted invalid module %+v", m)
			}
		}
		d.Origin.Hash = "short"
		if _, err := plan(Module{Path: modulePath, Version: version}, d); err == nil {
			t.Fatal("accepted short origin")
		}
	}
}
func TestBazelResolution(t *testing.T) {
	const hash = "17e27840396c1fa3196f184fcb5db1ab13850504"
	const sum = "f9c679f4e6b40eb3acdd864249b62c24da64be3bd7c02abf8be75d02f56ad415"
	url := "https://github.com/envoyproxy/envoy/archive/" + hash + ".tar.gz"
	wrap := func(body string) string {
		return `<query><rule class="http_archive" name="//external:envoy">` + body + `<string name="sha256" value="` + sum + `"/></rule></query>`
	}
	for _, body := range []string{`<string value="` + url + `" name="url"/>`, `<list name="urls"><string value="` + url + `"/></list>`} {
		p, err := bazelPlan(strings.NewReader(wrap(body)))
		if err != nil || p["commit"] != hash {
			t.Fatalf("%v %v", p, err)
		}
	}
	for _, input := range []string{`<query/>`, wrap(""), wrap(`<string name="url" value="https://example.com/archive.tar.gz"/>`), strings.ReplaceAll(wrap(`<string name="url" value="`+url+`"/>`), sum, ""), `<not-xml`, strings.ReplaceAll(wrap(`<string name="url" value="`+url+`"/>`), "http_archive", "git_repository")} {
		if _, err := bazelPlan(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted invalid query %s", input)
		}
	}
}
