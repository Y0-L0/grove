package workflow

import "fmt"

// DefaultType is the type of a workspace created without --type.
const DefaultType = "work"

// ValidateType checks the workspace types supported by this fork.
func ValidateType(kind string) error {
	if kind != "work" && kind != "priv" {
		return fmt.Errorf("invalid workspace type %q: choose work or priv", kind)
	}
	return nil
}
