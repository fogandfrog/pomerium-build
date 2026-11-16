// sourceplan resolves metadata using the upstream build tools, never source-text regexes.
package main

import (
	"bytes"
	"debug/elf"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const modulePath = "github.com/pomerium/envoy-custom"

var fullHash = regexp.MustCompile(`^[a-f0-9]{40}$`)

type Module struct {
	Path, Version, Sum, GoModSum, Error string
	Replace                             *Module
	Origin                              *struct{ VCS, URL, Hash string }
}
type Plan struct {
	Module   string `json:"module"`
	Version  string `json:"version"`
	Commit   string `json:"commit"`
	Sum      string `json:"sum"`
	GoModSum string `json:"go_mod_sum"`
}

func effective(m Module) (Module, error) {
	if m.Error != "" || m.Path != modulePath {
		return Module{}, fmt.Errorf("expected one %s module: %+v", modulePath, m)
	}
	if m.Replace != nil {
		m = *m.Replace
	}
	if m.Path != modulePath || m.Version == "" || m.Replace != nil {
		return Module{}, fmt.Errorf("unsupported local, foreign or nested Envoy replacement: %+v", m)
	}
	return m, nil
}
func plan(m, d Module) (Plan, error) {
	m, err := effective(m)
	if err != nil {
		return Plan{}, err
	}
	if d.Error != "" || d.Path != m.Path || d.Version != m.Version || d.Sum == "" || d.GoModSum == "" || d.Origin == nil {
		return Plan{}, fmt.Errorf("incomplete or inconsistent Go download metadata")
	}
	if d.Origin.VCS != "git" || d.Origin.URL != "https://"+modulePath || !fullHash.MatchString(d.Origin.Hash) {
		return Plan{}, fmt.Errorf("untrusted or incomplete Envoy origin: %+v", d.Origin)
	}
	if m.Sum != "" && m.Sum != d.Sum {
		return Plan{}, fmt.Errorf("module checksum mismatch")
	}
	return Plan{m.Path, m.Version, d.Origin.Hash, d.Sum, d.GoModSum}, nil
}
func workspace(dir string) (string, error) {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	file := filepath.Join(root, "go.work")
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return "off", nil
	} else if err != nil {
		return "", err
	}
	c := exec.Command("go", "work", "edit", "-json", file)
	c.Env = append(os.Environ(), "GOWORK="+file, "GOTOOLCHAIN=local")
	b, err := c.Output()
	if err != nil {
		return "", fmt.Errorf("parse Go workspace: %w", err)
	}
	var work struct{ Use []struct{ DiskPath string } }
	if err := json.Unmarshal(b, &work); err != nil {
		return "", err
	}
	if len(work.Use) == 0 {
		return "", fmt.Errorf("workspace has no local modules")
	}
	for _, use := range work.Use {
		path := use.DiskPath
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("workspace module escapes source checkout: %q", use.DiskPath)
		}
	}
	return file, nil
}
func goJSON(dir, work string, args ...string) (Module, error) {
	c := exec.Command("go", args...)
	c.Dir = dir
	c.Stderr = os.Stderr
	c.Env = append(os.Environ(), "GOWORK="+work, "GOTOOLCHAIN=local")
	b, err := c.Output()
	if err != nil {
		return Module{}, fmt.Errorf("go %v: %w", args, err)
	}
	var m Module
	if err = json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	return m, nil
}

// Bazel query XML is evaluated Starlark: variables, comments, aliases and file
// layout do not affect resolution. Unknown repository rules stop for review.
type Query struct {
	XMLName xml.Name `xml:"query"`
	Rules   []Rule   `xml:"rule"`
}
type Rule struct {
	Name    string  `xml:"name,attr"`
	Class   string  `xml:"class,attr"`
	Strings []Value `xml:"string"`
	Lists   []struct {
		Name   string  `xml:"name,attr"`
		Values []Value `xml:"string"`
	} `xml:"list"`
}
type Value struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

func bazelPlan(r io.Reader) (map[string]string, error) {
	// Bazel emits XML 1.1, while encoding/xml implements XML 1.0. Parse and
	// validate the declaration separately, then accept only the XML 1.0-compatible
	// subset of its payload. XML 1.1-only characters still fail closed.
	b, err := io.ReadAll(io.LimitReader(r, 4*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 4*1024*1024 {
		return nil, fmt.Errorf("oversized Bazel query result")
	}
	if bytes.HasPrefix(b, []byte("<?xml ")) {
		header, rest, ok := bytes.Cut(b, []byte("?>"))
		if !ok {
			return nil, fmt.Errorf("unterminated XML declaration")
		}
		var decl struct {
			Attributes []xml.Attr `xml:",any,attr"`
		}
		if err := xml.Unmarshal(append(append([]byte("<declaration "), header[len("<?xml "):]...), []byte("/>")...), &decl); err != nil {
			return nil, err
		}
		attrs := map[string]string{}
		for _, attr := range decl.Attributes {
			if _, exists := attrs[attr.Name.Local]; exists {
				return nil, fmt.Errorf("duplicate XML declaration attribute")
			}
			attrs[attr.Name.Local] = attr.Value
			if attr.Name.Space != "" || (attr.Name.Local != "version" && attr.Name.Local != "encoding" && attr.Name.Local != "standalone") {
				return nil, fmt.Errorf("unsupported XML declaration")
			}
		}
		if (attrs["version"] != "1.0" && attrs["version"] != "1.1") || (attrs["encoding"] != "" && !strings.EqualFold(attrs["encoding"], "UTF-8")) || (attrs["standalone"] != "" && attrs["standalone"] != "yes" && attrs["standalone"] != "no") {
			return nil, fmt.Errorf("unsupported XML version or encoding")
		}
		b = rest
	}
	var q Query
	decoder := xml.NewDecoder(bytes.NewReader(b))
	if err := decoder.Decode(&q); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("trailing or ambiguous Bazel query result")
	}
	if len(q.Rules) != 1 || q.Rules[0].Name != "//external:envoy" || q.Rules[0].Class != "http_archive" {
		return nil, fmt.Errorf("expected exactly one Envoy http_archive; new Bazel dependency scheme needs review")
	}
	attrs := map[string]string{}
	urls := []string{}
	for _, s := range q.Rules[0].Strings {
		if _, ok := attrs[s.Name]; ok {
			return nil, fmt.Errorf("duplicate attribute %s", s.Name)
		}
		attrs[s.Name] = s.Value
	}
	if attrs["url"] != "" {
		urls = append(urls, attrs["url"])
	}
	for _, l := range q.Rules[0].Lists {
		if l.Name == "urls" {
			for _, v := range l.Values {
				urls = append(urls, v.Value)
			}
		}
	}
	sha := regexp.MustCompile(`^[a-f0-9]{64}$`)
	urlPattern := regexp.MustCompile(`^https://github\.com/envoyproxy/envoy/archive/([a-f0-9]{40})\.tar\.gz$`)
	if len(urls) == 0 || !sha.MatchString(attrs["sha256"]) {
		return nil, fmt.Errorf("Envoy archive must have a SHA-256 and immutable source URL")
	}
	var commit string
	for _, u := range urls {
		m := urlPattern.FindStringSubmatch(u)
		if m == nil || (commit != "" && commit != m[1]) {
			return nil, fmt.Errorf("unsupported or conflicting Envoy source URL %q", u)
		}
		commit = m[1]
	}
	return map[string]string{"repository": "https://github.com/envoyproxy/envoy", "commit": commit, "archive_sha256": attrs["sha256"]}, nil
}
func run() (any, error) {
	if len(os.Args) == 3 && os.Args[1] == "license" {
		f, err := elf.Open(os.Args[2])
		if err != nil {
			return nil, err
		}
		defer f.Close()
		s := f.Section(".license")
		if s == nil {
			return nil, fmt.Errorf("missing Envoy license stamp")
		}
		b, err := s.Data()
		if err != nil {
			return nil, err
		}
		if strings.TrimRight(string(b), "\x00\n") != "Apache-2.0" {
			return nil, fmt.Errorf("unexpected Envoy license stamp %q", b)
		}
		return map[string]string{"license": "Apache-2.0"}, nil
	}
	if len(os.Args) == 2 && os.Args[1] == "bazel" {
		return bazelPlan(os.Stdin)
	}
	if len(os.Args) != 3 || os.Args[1] != "module" {
		return nil, fmt.Errorf("usage: sourceplan module UPSTREAM_DIR | sourceplan bazel < query.xml")
	}
	dir := os.Args[2]
	work, err := workspace(dir)
	if err != nil {
		return nil, err
	}
	m, err := goJSON(dir, work, "list", "-mod=readonly", "-m", "-json", modulePath)
	if err != nil {
		return nil, err
	}
	e, err := effective(m)
	if err != nil {
		return nil, err
	}
	d, err := goJSON(dir, work, "mod", "download", "-json", e.Path+"@"+e.Version)
	if err != nil {
		return nil, err
	}
	return plan(m, d)
}
func main() {
	p, err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err = json.NewEncoder(os.Stdout).Encode(p); err != nil {
		panic(err)
	}
}
