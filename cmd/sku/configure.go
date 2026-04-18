package sku

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/sofq/sku/internal/config"
)

// newConfigureCmd wires `sku configure`. It runs in either flagged mode —
// when any per-field flag below was explicitly `Changed()` — or interactive
// mode — when stdin is a TTY and no per-field flag was set. If stdin is not
// a TTY and no per-field flag was set, the command warns on stderr and
// exits with a validation error.
//
// The `--profile` selector is inherited from root. The target profile name
// defaults to "default" when the root flag is unset.
//
// Overrides root's PersistentPreRunE so the pre-run doesn't fail when the
// operator asks to write a not-yet-existing profile (e.g. `--profile ci`).
func newConfigureCmd() *cobra.Command {
	var (
		channel          string
		defaultRegions   []string
		staleWarningDays int
		staleErrorDays   int
	)

	c := &cobra.Command{
		Use:   "configure",
		Short: "Create or edit a named config profile",
		Args:  cobra.NoArgs,
		// Overrides root.PersistentPreRunE to skip profile-existence checks.
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags := cmd.Flags()

			// Profile name: fall back to "default" when root --profile unset.
			profileName := "default"
			if fl := flags.Lookup("profile"); fl != nil && fl.Value.String() != "" {
				profileName = fl.Value.String()
			}

			// Load existing so we don't clobber fields absent from the
			// current invocation — the edited profile is a merge of the
			// old profile and any per-field flag that was Changed().
			path := config.Path()
			existing, err := config.Load(path)
			if err != nil {
				return err
			}
			p := existing.Profiles[profileName]

			anyFieldSet := false
			changed := func(name string) bool {
				fl := flags.Lookup(name)
				if fl == nil {
					return false
				}
				if fl.Changed {
					anyFieldSet = true
					return true
				}
				return false
			}

			if changed("preset") {
				if fl := flags.Lookup("preset"); fl != nil {
					p.Preset = fl.Value.String()
				}
			}
			if changed("channel") {
				p.Channel = channel
			}
			if changed("default-regions") {
				p.DefaultRegions = defaultRegions
			}
			if changed("stale-warning-days") {
				v := staleWarningDays
				p.StaleWarningDays = &v
			}
			if changed("stale-error-days") {
				v := staleErrorDays
				p.StaleErrorDays = &v
			}
			if changed("auto-fetch") {
				if fl := flags.Lookup("auto-fetch"); fl != nil {
					b := fl.Value.String() == "true"
					p.AutoFetch = &b
				}
			}
			if changed("include-raw") {
				if fl := flags.Lookup("include-raw"); fl != nil {
					b := fl.Value.String() == "true"
					p.IncludeRaw = &b
				}
			}

			if !anyFieldSet {
				// Interactive mode — requires a TTY on stdin.
				if !stdinIsTTY(cmd.InOrStdin()) {
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(),
						"configure: no --preset/--stale-*/--auto-fetch/... flags set and stdin is not a TTY; nothing to do")
					return fmt.Errorf("configure: no fields to write")
				}
				if err := promptProfile(cmd.InOrStdin(), cmd.OutOrStdout(), &p); err != nil {
					return err
				}
			}

			if err := config.SaveProfile(path, profileName, p); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "wrote profile %q to %s\n", profileName, path)
			return nil
		},
	}

	f := c.Flags()
	f.StringVar(&channel, "channel", "", "release channel (stable|edge)")
	f.StringSliceVar(&defaultRegions, "default-regions", nil, "comma-separated default regions")
	f.IntVar(&staleWarningDays, "stale-warning-days", 14, "warn when catalog is this many days old")
	f.IntVar(&staleErrorDays, "stale-error-days", 0, "error when catalog is this many days old (0=disabled)")
	return c
}

// promptProfile walks the caller through each editable field. Empty input
// keeps the current value. Booleans accept y/n/true/false; empty keeps
// current.
func promptProfile(in io.Reader, out io.Writer, p *config.Profile) error {
	sc := bufio.NewScanner(in)
	ask := func(label, current string) string {
		_, _ = fmt.Fprintf(out, "%s [%s]: ", label, current)
		if !sc.Scan() {
			return ""
		}
		return strings.TrimSpace(sc.Text())
	}

	if v := ask("preset (agent|full|price|compare)", p.Preset); v != "" {
		p.Preset = v
	}
	if v := ask("channel (stable|edge)", p.Channel); v != "" {
		p.Channel = v
	}
	if v := ask("default-regions (comma-separated)", strings.Join(p.DefaultRegions, ",")); v != "" {
		p.DefaultRegions = splitCSV(v)
	}
	if v := ask("stale-warning-days", intPtrStr(p.StaleWarningDays)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.StaleWarningDays = &n
		}
	}
	if v := ask("stale-error-days (0=disabled)", intPtrStr(p.StaleErrorDays)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.StaleErrorDays = &n
		}
	}
	if v := ask("auto-fetch (y/n)", boolPtrStr(p.AutoFetch)); v != "" {
		b := parseYN(v)
		p.AutoFetch = &b
	}
	if v := ask("include-raw (y/n)", boolPtrStr(p.IncludeRaw)); v != "" {
		b := parseYN(v)
		p.IncludeRaw = &b
	}
	return nil
}

func stdinIsTTY(in io.Reader) bool {
	f, ok := in.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd())) //nolint:gosec // Fd() is a small non-negative uintptr
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func intPtrStr(p *int) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(*p)
}

func boolPtrStr(p *bool) string {
	if p == nil {
		return ""
	}
	if *p {
		return "y"
	}
	return "n"
}

func parseYN(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "y", "yes", "true", "on":
		return true
	}
	return false
}
