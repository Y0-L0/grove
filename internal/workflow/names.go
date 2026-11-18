package workflow

import (
	"fmt"
	"path/filepath"

	"github.com/nicksenap/grove/internal/gitops"
	"github.com/nicksenap/grove/internal/models"
	"github.com/nicksenap/grove/internal/operations"
)

// CheckCreate reserves names with committed records across all configured types.
func (Hooks) CheckCreate(req operations.CreateRequest) error {
	return checkRecordsName(req.Options.Cfg, req.Name)
}

func checkRecordsName(cfg *models.Config, name string) error {
	if cfg == nil {
		return nil
	}
	for _, kind := range cfg.Types {
		if kind.Context == "" {
			continue
		}
		source, err := contextPath(kind.Context)
		if err != nil {
			return err
		}
		base, err := gitops.ResolveLocalBaseBranch(source)
		if err != nil {
			return err
		}
		paths, err := gitops.TreePaths(source, base, filepath.ToSlash(filepath.Join("records", name)))
		if err != nil {
			return err
		}
		if paths != "" {
			return fmt.Errorf("records already exist for %s; use gw reopen %s", name, name)
		}
	}
	return nil
}
