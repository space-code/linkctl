package aasa

import (
	"fmt"
	"strings"

	"github.com/space-code/linkctl/internal/pbxproj"
)

// IdentityFromProject reads an Xcode project and returns the AppIdentity
// (TeamID + BundleID) for one of its targets, so `aasa`/`onelink` can be
// pointed at --project instead of requiring --bundle-id/--team-id by hand.
//
// targetFilter selects a specific target by name (case-insensitive); when
// empty and the project has exactly one native target, that target is used.
// When empty and there are multiple targets, an error lists their names so
// the caller knows to pass --target.
func IdentityFromProject(projectPath, targetFilter string) (AppIdentity, error) {
	proj, err := pbxproj.ParseXcodeProject(projectPath, "")
	if err != nil {
		return AppIdentity{}, err
	}

	if len(proj.Targets) == 0 {
		return AppIdentity{}, fmt.Errorf("no targets found in %q", projectPath)
	}

	if targetFilter != "" {
		for _, t := range proj.Targets {
			if strings.EqualFold(t.Name, targetFilter) {
				return AppIdentity{TeamID: t.TeamID, BundleID: t.BundleID}, nil
			}
		}
		names := make([]string, len(proj.Targets))
		for i, t := range proj.Targets {
			names[i] = t.Name
		}
		return AppIdentity{}, fmt.Errorf("target %q not found; available targets: %s", targetFilter, strings.Join(names, ", "))
	}

	if len(proj.Targets) > 1 {
		names := make([]string, len(proj.Targets))
		for i, t := range proj.Targets {
			names[i] = t.Name
		}
		return AppIdentity{}, fmt.Errorf("project has multiple targets, pass --target: %s", strings.Join(names, ", "))
	}

	t := proj.Targets[0]
	return AppIdentity{TeamID: t.TeamID, BundleID: t.BundleID}, nil
}
