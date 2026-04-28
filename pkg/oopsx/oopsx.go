// Package oopsx wraps github.com/samber/oops with domain "orch" and optional subsystem tags.
// Prefer B("subsystem").Wrapf / Errorf over fmt.Errorf so errors carry structured context.
//
// Note: github.com/hashicorp/raft StableStore compares err.Error() == "not found" for empty keys;
// keep plain errors.New("not found") there — do not route those through oops.
//
// This package lives under pkg/ so other modules may depend on it without importing internal/.
package oopsx

import "github.com/samber/oops"

const Domain = "orch"

// B returns an oops builder with In("orch") and optional Tags(tags...).
func B(tags ...string) oops.OopsErrorBuilder {
	if len(tags) == 0 {
		return oops.In(Domain)
	}
	return oops.In(Domain).Tags(tags...)
}
