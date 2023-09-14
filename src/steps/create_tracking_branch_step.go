package steps

import (
	"github.com/git-town/git-town/v9/src/domain"
	"github.com/git-town/git-town/v9/src/git"
	"github.com/git-town/git-town/v9/src/hosting"
)

// CreateTrackingBranchStep pushes the given local branch up to origin
// and marks it as tracking the current branch.
type CreateTrackingBranchStep struct {
	Branch     domain.LocalBranchName
	NoPushHook bool
	EmptyStep
}

func (step *CreateTrackingBranchStep) CreateUndoSteps(_ *git.BackendCommands) ([]Step, error) {
	return []Step{&DeleteRemoteBranchStep{Branch: step.Branch, NoPushHook: false}}, nil
}

func (step *CreateTrackingBranchStep) Run(run *git.ProdRunner, _ hosting.Connector) error {
	return run.Frontend.CreateTrackingBranch(step.Branch, domain.OriginRemote, step.NoPushHook)
}
