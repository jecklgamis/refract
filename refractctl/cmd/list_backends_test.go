package cmd

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/jecklgamis/refract/refractctl/internal/config"
	ec "github.com/jecklgamis/refract/refractctl/internal/envoyconfig"
	"github.com/jecklgamis/refract/refractctl/internal/render"
)

func TestChooseListSource(t *testing.T) {
	cases := []struct {
		name          string
		local, remote bool
		mode          string
		want          listSource
	}{
		{"no flags, no mode -> local", false, false, "", sourceLocal},
		{"no flags, http mode -> remote", false, false, "http", sourceRemote},
		{"no flags, s3 mode -> remote", false, false, "s3", sourceRemote},
		{"--local beats a configured mode", true, false, "http", sourceLocal},
		{"--remote without a mode still asks remote", false, true, "", sourceRemote},
		{"--remote with a mode -> remote", false, true, "s3", sourceRemote},
	}
	for _, c := range cases {
		if got := chooseListSource(c.local, c.remote, c.mode); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// renderThenParseCDS mirrors renderThenParseLDS: values -> render.BuildCDS ->
// yaml.Marshal (what gets pushed) -> yaml.Unmarshal (what a remote fetch
// sees come back).
func renderThenParseCDS(t *testing.T, v config.Values) ec.CDS {
	t.Helper()
	body, err := yaml.Marshal(render.BuildCDS(v))
	if err != nil {
		t.Fatalf("marshaling CDS: %v", err)
	}
	var cds ec.CDS
	if err := yaml.Unmarshal(body, &cds); err != nil {
		t.Fatalf("unmarshaling CDS: %v\n%s", err, body)
	}
	return cds
}

func sampleValues() config.Values {
	return config.Values{Backends: []config.Backend{
		{Name: "httpbin", Host: "httpbin.org", Port: 443, TLS: true, RoutePrefix: "/httpbin/", Timeout: "15s"},
		{Name: "svc-a", Host: "svc-a.internal", Port: 8080, Domain: "frontend-a.test.local", Timeout: "15s"},
		{Name: "scoped", Host: "10.0.0.5", Port: 9000, Domain: "api.test.local", RoutePrefix: "/v1/", Timeout: "15s"},
		{Name: "cluster-only", Host: "10.0.0.6", Port: 7000},
	}}
}

// What was pushed must read back as identical rows, otherwise every
// list-backends would cry drift on a perfectly healthy deployment.
func TestRemoteRowsMatchLocalRowsAfterRoundTrip(t *testing.T) {
	v := sampleValues()
	local := localRows(v)
	remote := remoteRows(renderThenParseCDS(t, v), renderThenParseLDS(t, v))

	onlyLocal, onlyRemote, changed := diffRows(local, remote)
	if len(onlyLocal)+len(onlyRemote)+len(changed) != 0 {
		t.Fatalf("round trip reported drift: onlyLocal=%v onlyRemote=%v changed=%v\nlocal=%v\nremote=%v",
			onlyLocal, onlyRemote, changed, local, remote)
	}
	if len(remote) != len(v.Backends) {
		t.Errorf("got %d remote rows, want %d (envoy_admin/default_app must be skipped)", len(remote), len(v.Backends))
	}
}

func TestDiffRows(t *testing.T) {
	local := []backendRow{
		{Name: "a", Host: "a.test", Port: 80, Route: "/a/"},
		{Name: "b", Host: "b.test", Port: 80, Route: "/b/"},
		{Name: "only-local", Host: "x", Port: 1, Route: "/x/"},
	}
	remote := []backendRow{
		{Name: "a", Host: "a.test", Port: 80, Route: "/a/"},
		{Name: "b", Host: "b.test", Port: 8080, Route: "/b/"}, // port differs
		{Name: "only-live", Host: "y", Port: 2, Route: "/y/"},
	}
	onlyLocal, onlyRemote, changed := diffRows(local, remote)
	if !reflect.DeepEqual(onlyLocal, []string{"only-local"}) {
		t.Errorf("onlyLocal = %v", onlyLocal)
	}
	if !reflect.DeepEqual(onlyRemote, []string{"only-live"}) {
		t.Errorf("onlyRemote = %v", onlyRemote)
	}
	if !reflect.DeepEqual(changed, []string{"b"}) {
		t.Errorf("changed = %v", changed)
	}
}

// withValuesFile points valuesPath at a values.yaml holding v (or at a
// nonexistent file when v is nil) for the duration of the test.
func withValuesFile(t *testing.T, v *config.Values) {
	t.Helper()
	old := valuesPath
	t.Cleanup(func() { valuesPath = old })
	valuesPath = filepath.Join(t.TempDir(), "values.yaml")
	if v != nil {
		if err := config.Save(valuesPath, *v); err != nil {
			t.Fatalf("saving values: %v", err)
		}
	}
}

func TestReportDriftSilentWhenInSync(t *testing.T) {
	v := sampleValues()
	withValuesFile(t, &v)
	var buf bytes.Buffer
	reportDrift(&buf, localRows(v))
	if buf.Len() != 0 {
		t.Errorf("expected no warning, got:\n%s", buf.String())
	}
}

func TestReportDriftNamesTheDifferences(t *testing.T) {
	v := sampleValues()
	withValuesFile(t, &v)
	live := localRows(v)
	live = live[1:]                                                    // "httpbin" missing from live
	live = append(live, backendRow{Name: "ghost", Host: "g", Port: 1}) // "ghost" only live
	live[0].Port = 1234                                                // "svc-a" differs

	var buf bytes.Buffer
	reportDrift(&buf, live)
	out := buf.String()
	for _, want := range []string{
		"only live (not in values.yaml): ghost",
		"only in values.yaml (not live): httpbin",
		"differs: svc-a",
		"generate-values --force",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("warning missing %q; got:\n%s", want, out)
		}
	}
}

func TestReportDriftNoLocalFile(t *testing.T) {
	withValuesFile(t, nil)
	var buf bytes.Buffer
	reportDrift(&buf, []backendRow{{Name: "a"}, {Name: "b"}})
	out := buf.String()
	if !strings.Contains(out, "no local") || !strings.Contains(out, "2 backend(s) are live") || !strings.Contains(out, "generate-values") {
		t.Errorf("unexpected warning:\n%s", out)
	}

	buf.Reset()
	reportDrift(&buf, nil) // nothing live and no file: nothing to warn about
	if buf.Len() != 0 {
		t.Errorf("expected silence with nothing live and no local file, got:\n%s", buf.String())
	}
}

func TestShouldNoteNoMode(t *testing.T) {
	cases := []struct {
		name          string
		local, remote bool
		mode          string
		want          bool
	}{
		{"no flags, no mode -> note", false, false, "", true},
		{"no flags, mode set -> no note (reads remote)", false, false, "http", false},
		{"--local, no mode -> no note (asked for it)", true, false, "", false},
		{"--remote, no mode -> no note", false, true, "", false},
	}
	for _, c := range cases {
		if got := shouldNoteNoMode(c.local, c.remote, c.mode); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestListLocalBackendsErrorsOnMissingFile(t *testing.T) {
	withValuesFile(t, nil)
	err := listLocalBackends()
	if err == nil {
		t.Fatal("expected an error for a missing values.yaml, got nil")
	}
	if !strings.Contains(err.Error(), valuesPath) || !strings.Contains(err.Error(), "--values") {
		t.Errorf("error should name the path and the --values flag, got: %v", err)
	}
}

func TestListLocalBackendsOKOnEmptyExistingFile(t *testing.T) {
	v := config.Values{Backends: []config.Backend{}}
	withValuesFile(t, &v)
	if err := listLocalBackends(); err != nil {
		t.Errorf("an existing empty values.yaml is a legitimate empty list, got error: %v", err)
	}
}
