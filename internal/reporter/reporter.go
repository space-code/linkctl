package reporter

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/space-code/linkctl/internal/models"
	"github.com/space-code/linkctl/pkg/iostreams"
)

func PrintBanner(w io.Writer) {
	fmt.Println(w, "Debugger")
}

func PrintDeviceList(w io.Writer, cs *iostreams.ColorScheme, platform string, devices []string) {
	if len(devices) == 0 {
		fmt.Fprintf(w, "  %s  No %s devices found (booted / connected)\n\n", "⚠️", platform)
		return
	}

	fmt.Fprintf(w, "%s\n", cs.Bold(platform))
	for _, d := range devices {
		fmt.Fprintf(w, "%s %s\n", cs.Muted("•"), d)
	}
	fmt.Fprintln(w)
}

func PrintAppCheckReport(w io.Writer, cs *iostreams.ColorScheme, report *models.AppCheckReport) {
	// Header
	icon := "🍎"
	if report.Platform == "android" {
		icon = "🤖"
	}
	fmt.Fprintf(w, " %s APP CONFIG CHECK [%s]\n\n", icon, strings.ToUpper(report.Platform))

	// Project metadata
	if report.XcodeProject != nil {
		names := make([]string, 0, len(report.XcodeProject.Targets))
		for _, t := range report.XcodeProject.Targets {
			names = append(names, t.Name)
		}
		printKV(w, cs, [][]string{
			{"Project", report.XcodeProject.Path},
			{"Targets", strings.Join(names, ", ")},
		})
		fmt.Fprintln(w)
	}
	if report.AndroidManifest != nil {
		printKV(w, cs, [][]string{
			{"Manifest", report.AndroidManifest.Path},
			{"Package", report.AndroidManifest.PackageName},
		})
		fmt.Fprintln(w)
	}

	// Checks — group by target prefix so multi-target output is scannable.
	printChecks(w, cs, report.Checks)

	// Summary line
	fmt.Fprintf(
		w, "\n  Summary: %s   %s   %s\n\n",
		cs.Green(fmt.Sprintf("%d passed", report.Summary.Passed)),
		cs.Red(fmt.Sprintf("%d failed", report.Summary.Failed)),
		cs.Yellow(fmt.Sprintf("%d warnings", report.Summary.Warnings)),
	)

	if report.Summary.OK {
		fmt.Fprintf(w, "  %s  App is correctly configured!\n\n", cs.Green("✅"))
	} else {
		fmt.Fprintf(w, "  %s  Configuration has issues — deep link may not work.\n\n", cs.Red("✗"))
	}
}

func printChecks(w io.Writer, cs *iostreams.ColorScheme, checks []models.ValidationResult) {
	if len(checks) == 0 {
		return
	}

	// Measure longest check name for alignment.
	maxWidth := 0
	for _, r := range checks {
		if n := utf8.RuneCountInString(r.Check); n > maxWidth {
			maxWidth = n
		}
	}
	// Cap at 52 so very long names don't push messages off-screen.
	if maxWidth > 52 {
		maxWidth = 52
	}

	for _, r := range checks {
		name := r.Check
		pad := maxWidth - utf8.RuneCountInString(name)
		if pad < 0 {
			pad = 0
		}
		fmt.Fprintf(
			w, "  %s  %s%s  %s\n",
			StatusIcon(cs, r.Status),
			name,
			strings.Repeat(" ", pad),
			r.Message,
		)
		if r.Detail != "" {
			fmt.Fprintf(w, "        %s %s\n", cs.Muted("↳"), r.Detail)
		}
	}
}

func printKV(w io.Writer, cs *iostreams.ColorScheme, rows [][]string) {
	maxKey := 0
	for _, row := range rows {
		if len(row) >= 1 {
			if n := utf8.RuneCountInString(row[0]); n > maxKey {
				maxKey = n
			}
		}
	}
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		pad := maxKey - utf8.RuneCountInString(row[0])
		fmt.Fprintf(
			w, "  %s%s  %s\n",
			cs.Muted(row[0]+":"),
			strings.Repeat(" ", pad),
			row[1],
		)
	}
}

func StatusIcon(cs *iostreams.ColorScheme, s models.Status) string {
	switch s {
	case models.StatusPass:
		return cs.Green("✓")
	case models.StatusFail:
		return cs.Red("✗")
	case models.StatusWarning:
		return cs.Yellow("⚠")
	case models.StatusInfo:
		return cs.Cyan("ℹ")
	default:
		return cs.Muted("–")
	}
}

func PrintLinkInfo(w io.Writer, cs *iostreams.ColorScheme, link *models.DeepLink) {
	fmt.Fprintln(w, " 🔍 PARSED LINK")
	fmt.Fprintln(w)

	rows := [][]string{
		{"Type", string(link.Type)},
		{"Scheme", link.Scheme},
		{"Host", link.Host},
	}
	if link.Path != "" && link.Path != "/" {
		rows = append(rows, []string{"Path", link.Path})
	}

	// Query params — sorted for deterministic output.
	if len(link.Query) > 0 {
		keys := make([]string, 0, len(link.Query))
		for k := range link.Query {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			rows = append(rows, []string{"  ?" + k, link.Query[k]})
		}
	}
	if link.Fragment != "" {
		rows = append(rows, []string{"Fragment", link.Fragment})
	}

	printKV(w, cs, rows)
	fmt.Fprintln(w)
}

func PrintProjectScan(w io.Writer, cs *iostreams.ColorScheme, scan *models.ProjectScan) {
	icon := "🍎"
	if scan.Platform == "android" {
		icon = "🤖"
	}
	fmt.Fprintf(w, " %s PROJECT SCAN [%s]\n\n", icon, strings.ToUpper(scan.Platform))
	printKV(w, cs, [][]string{{"Project", scan.ProjectPath}})
	fmt.Fprintln(w)

	fmt.Fprintln(w, " 🎯 TARGETS")
	fmt.Fprintln(w)
	for _, t := range scan.Targets {
		fmt.Fprintf(w, "  %s\n", cs.Bold(t.Name))

		lastField := lastNonEmpty(
			len(t.AssociatedDomains) > 0,
			len(t.URLSchemes) > 0,
			t.EntitlementsPath != "",
			t.InfoPlistPath != "",
		)
		fields := 0

		if len(t.AssociatedDomains) > 0 {
			fields++
			branch := treeNode(fields, lastField)
			fmt.Fprintf(w, "  %s Associated Domains:  %s\n",
				branch, strings.Join(t.AssociatedDomains, ", "))
		}
		if len(t.URLSchemes) > 0 {
			fields++
			branch := treeNode(fields, lastField)
			fmt.Fprintf(w, "  %s URL Schemes:         %s\n",
				branch, strings.Join(t.URLSchemes, ", "))
		}
		if t.EntitlementsPath != "" {
			fields++
			branch := treeNode(fields, lastField)
			fmt.Fprintf(w, "  %s Entitlements:        %s\n",
				branch, cs.Muted(t.EntitlementsPath))
		}
		if t.InfoPlistPath != "" {
			fields++
			branch := treeNode(fields, lastField)
			fmt.Fprintf(w, "  %s Info.plist:          %s\n",
				branch, cs.Muted(t.InfoPlistPath))
		}
		fmt.Fprintln(w)
	}

	if len(scan.RegisteredLinks) > 0 {
		fmt.Fprintln(w, " 🔗 REGISTERED LINK PATTERNS")
		fmt.Fprintln(w)
		for _, l := range scan.RegisteredLinks {
			var label string
			switch l.LinkType {
			case "Universal Link", "App Link":
				label = cs.Green(padRight(l.LinkType, 16))
			case "Custom Scheme":
				label = cs.Yellow(padRight(l.LinkType, 16))
			default:
				label = padRight(l.LinkType, 16)
			}
			fmt.Fprintf(w, "  %s  %-42s → %s\n", label, l.Pattern, cs.Muted(l.Target))
		}
		fmt.Fprintln(w)
	} else {
		fmt.Fprintf(w, "  %s  No deep link patterns registered in this project.\n\n", cs.Yellow("⚠"))
	}
}

func padRight(s string, w int) string {
	pad := w - utf8.RuneCountInString(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

func treeNode(n, last int) string {
	if n == last {
		return "└──"
	}
	return "├──"
}

func lastNonEmpty(fields ...bool) int {
	count := 0
	for _, f := range fields {
		if f {
			count++
		}
	}
	return count
}
