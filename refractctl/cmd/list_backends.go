package cmd

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jecklgamis/refract/refractctl/internal/config"
	ec "github.com/jecklgamis/refract/refractctl/internal/envoyconfig"
)

var (
	lbRemote bool
	lbLocal  bool
)

var listBackendsCmd = &cobra.Command{
	Use:   "list-backends",
	Short: "List backends that are live (when a mode is configured) or in values.yaml (otherwise)",
	Long: `List backends.

When a distribution mode is configured (CONFIG_SOURCE_KIND or the settings
file's "mode"), this lists what is actually live on the config server or S3,
since the local values.yaml can drift from it (a push that failed, or a
different machine/directory pushed last). If values.yaml differs from what's
live, a warning on stderr names the difference.

With no mode configured it reads the local values.yaml. Use --local or
--remote to force either source.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if lbLocal && lbRemote {
			return fmt.Errorf("--local and --remote are mutually exclusive")
		}
		mode := resolveMode()
		if chooseListSource(lbLocal, lbRemote, mode) == sourceLocal {
			if err := listLocalBackends(); err != nil {
				return err
			}
			// Only after a successful listing: the note says what was shown.
			if shouldNoteNoMode(lbLocal, lbRemote, mode) {
				fmt.Fprintln(cmd.ErrOrStderr(), noModeNote)
			}
			return nil
		}
		return listRemoteBackends(cmd.ErrOrStderr())
	},
}

type listSource int

const (
	sourceLocal listSource = iota
	sourceRemote
)

// chooseListSource decides where list-backends reads from: an explicit
// --local/--remote always wins; otherwise a configured mode means the live
// config is the source of truth and no mode means there is nothing remote
// to ask.
func chooseListSource(local, remote bool, mode string) listSource {
	switch {
	case local:
		return sourceLocal
	case remote:
		return sourceRemote
	case mode != "":
		return sourceRemote
	default:
		return sourceLocal
	}
}

const noModeNote = "note: no mode configured, showing local values.yaml (may not match what's live). " +
	"Set one with: refractctl config set mode http|s3"

// shouldNoteNoMode reports whether list-backends fell back to the local
// file only because no mode is configured (not because --local asked for
// it), in which case it says so on stderr - the list is just this one file
// and could otherwise be mistaken for the live state.
func shouldNoteNoMode(local, remote bool, mode string) bool {
	return !local && !remote && mode == ""
}

// backendRow is one line of list-backends output, built from either local
// values.yaml or the live cds.yaml/lds.yaml so the two can be printed and
// compared the same way.
type backendRow struct {
	Name  string
	Host  string
	Port  int
	TLS   bool
	Route string
}

func (r backendRow) String() string {
	tls := "plaintext"
	if r.TLS {
		tls = "tls"
	}
	return fmt.Sprintf("%-20s %s:%-6d %-10s %s", r.Name, r.Host, r.Port, tls, r.Route)
}

func routeLabel(domain, prefix string) string {
	switch {
	case domain != "" && prefix != "":
		return domain + prefix
	case domain != "":
		return domain
	case prefix != "":
		return prefix
	default:
		return "(no route, cluster only)"
	}
}

func localRows(v config.Values) []backendRow {
	rows := make([]backendRow, 0, len(v.Backends))
	for _, b := range v.Backends {
		rows = append(rows, backendRow{
			Name: b.Name, Host: b.Host, Port: b.Port, TLS: b.TLS,
			Route: routeLabel(b.Domain, b.RoutePrefix),
		})
	}
	return rows
}

// remoteRows reconstructs the same rows from what's actually live: the
// config server's/S3's cds.yaml and lds.yaml, the same files the
// in-container fetcher polls.
func remoteRows(cds ec.CDS, lds ec.LDS) []backendRow {
	var rows []backendRow
	for _, c := range cds.Resources {
		if c.Name == "envoy_admin" || c.Name == "default_app" {
			continue
		}
		host, port := "?", 0
		if len(c.LoadAssignment.Endpoints) > 0 && len(c.LoadAssignment.Endpoints[0].LBEndpoints) > 0 {
			addr := c.LoadAssignment.Endpoints[0].LBEndpoints[0].Endpoint.Address.SocketAddress
			host, port = addr.Address, addr.PortValue
		}
		route := routeLabel("", "")
		if info, ok := findRemoteRoute(lds, c.Name); ok {
			route = routeLabel(info.Domain, info.RoutePrefix)
		}
		rows = append(rows, backendRow{
			Name: c.Name, Host: host, Port: port,
			TLS: c.TransportSocket != nil, Route: route,
		})
	}
	return rows
}

// diffRows compares local and live rows by backend name and reports which
// names exist only on one side, plus those on both sides whose host, port,
// TLS or route differ.
func diffRows(local, remote []backendRow) (onlyLocal, onlyRemote, changed []string) {
	l := make(map[string]backendRow, len(local))
	for _, row := range local {
		l[row.Name] = row
	}
	r := make(map[string]backendRow, len(remote))
	for _, row := range remote {
		r[row.Name] = row
	}
	for name, lr := range l {
		rr, ok := r[name]
		switch {
		case !ok:
			onlyLocal = append(onlyLocal, name)
		case lr != rr:
			changed = append(changed, name)
		}
	}
	for name := range r {
		if _, ok := l[name]; !ok {
			onlyRemote = append(onlyRemote, name)
		}
	}
	sort.Strings(onlyLocal)
	sort.Strings(onlyRemote)
	sort.Strings(changed)
	return
}

func printRows(rows []backendRow) {
	if len(rows) == 0 {
		fmt.Println("No backends configured")
		return
	}
	for _, r := range rows {
		fmt.Println(r)
	}
}

func listLocalBackends() error {
	// config.Load treats a missing file as "no backends", which here would
	// read as a confident empty answer even when this is just the wrong
	// directory - so a missing file is an error for a listing.
	if _, err := os.Stat(valuesPath); os.IsNotExist(err) {
		return fmt.Errorf("no values.yaml at %s (run from the directory that holds it, "+
			"pass --values, or configure a mode to list what's live)", valuesPath)
	}
	v, err := config.Load(valuesPath)
	if err != nil {
		return err
	}
	printRows(localRows(v))
	return nil
}

// listRemoteBackends prints what's actually live. Local and live can
// disagree (a push that failed silently, or values.yaml here not being the
// one that was last used to push) - that drift is what this exists to
// surface, so after the table it warns on w if values.yaml differs.
func listRemoteBackends(w io.Writer) error {
	cds, lds, found, err := fetchRemoteCDSAndLDS()
	if err != nil {
		return fmt.Errorf("fetching the live config: %w (use --local to list values.yaml instead)", err)
	}
	var live []backendRow
	if !found {
		fmt.Println("No config on the remote source yet (nothing pushed)")
	} else {
		live = remoteRows(cds, lds)
		printRows(live)
	}
	reportDrift(w, live)
	return nil
}

// reportDrift compares the live rows against the local values.yaml and
// writes a warning to w if they differ. It never fails the command: the
// listing already succeeded.
func reportDrift(w io.Writer, live []backendRow) {
	if _, err := os.Stat(valuesPath); os.IsNotExist(err) {
		if len(live) > 0 {
			fmt.Fprintf(w, "\nwarning: no local %s here, but %d backend(s) are live.\n"+
				"To create it from what's live: refractctl generate-values\n", valuesPath, len(live))
		}
		return
	}
	v, err := config.Load(valuesPath)
	if err != nil {
		fmt.Fprintf(w, "\nwarning: could not read %s to compare against what's live: %v\n", valuesPath, err)
		return
	}
	onlyLocal, onlyLive, changed := diffRows(localRows(v), live)
	if len(onlyLocal)+len(onlyLive)+len(changed) == 0 {
		return
	}
	fmt.Fprintf(w, "\nwarning: %s differs from what's live:\n", valuesPath)
	if len(onlyLive) > 0 {
		fmt.Fprintf(w, "  only live (not in values.yaml): %s\n", strings.Join(onlyLive, ", "))
	}
	if len(onlyLocal) > 0 {
		fmt.Fprintf(w, "  only in values.yaml (not live): %s\n", strings.Join(onlyLocal, ", "))
	}
	if len(changed) > 0 {
		fmt.Fprintf(w, "  differs: %s\n", strings.Join(changed, ", "))
	}
	fmt.Fprintln(w, "To rebuild values.yaml from what's live: refractctl generate-values --force")
}

func init() {
	listBackendsCmd.Flags().BoolVar(&lbRemote, "remote", false,
		"List what's actually live on the config server or S3 (the default when a mode is configured).")
	listBackendsCmd.Flags().BoolVar(&lbLocal, "local", false,
		"List the local values.yaml instead, even when a mode is configured.")
	rootCmd.AddCommand(listBackendsCmd)
}
