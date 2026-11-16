package operations_test

import (
	"testing"

	"github.com/nicksenap/grove/internal/models"
	"github.com/nicksenap/grove/internal/operations"
	"github.com/nicksenap/grove/internal/workflow"
	"github.com/nicksenap/grove/internal/workspace"
)

func TestDefaultWorkspaceBranch(t *testing.T) {
	svc := operations.Service{Hooks: []operations.Hooks{workflow.Hooks{}}}
	req, err := svc.ResolveBranch(operations.CreateRequest{
		Name: "login",
		Options: workspace.CreateOpts{Cfg: &models.Config{
			OwnerPrefix: "alice",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.Options.Branch != "alice/login" {
		t.Fatalf("branch = %q, want alice/login", req.Options.Branch)
	}
}
