// Package scope decides whether one path is inside the lock.
package scope

import (
	"fmt"

	"github.com/vaquarkhan/ProofSieve/internal/globset"
	"github.com/vaquarkhan/ProofSieve/internal/lockfile"
)

// Decision is the allow or deny result for a path.
type Decision struct {
	Allow   bool
	Rule    string
	Message string
}

// Decide applies deny globs, allow globs, and budgets.
// policy paths are always denied. files is the count of other files already charged.
// loc is the churn already charged, plus this write.
func Decide(lock lockfile.Lock, rel string, files, loc int, newFile bool) Decision {
	if lockfile.IsPolicyPath(rel) {
		return deny("policy-path", rel+" is inside .proofsieve and only a human may edit it")
	}
	denied, err := globset.Compile(lock.Blast.Denied)
	if err != nil {
		return deny("lock", err.Error())
	}
	if denied.Match(rel) {
		return deny("denied-path", rel+" matches a denied path")
	}
	allowed, err := globset.Compile(lock.Blast.Allowed)
	if err != nil {
		return deny("lock", err.Error())
	}
	if !allowed.Match(rel) {
		return deny("outside-scope", rel+" is outside the scope lock")
	}
	if newFile && files+1 > lock.Blast.MaxFiles {
		return deny("max-files", fmt.Sprintf("scope lock allows %d files", lock.Blast.MaxFiles))
	}
	if loc > lock.Blast.MaxLoc {
		return deny("max-loc", fmt.Sprintf("scope lock allows %d lines of churn", lock.Blast.MaxLoc))
	}
	return Decision{Allow: true}
}

func deny(rule, msg string) Decision {
	return Decision{Rule: rule, Message: msg}
}
