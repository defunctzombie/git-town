package config

import (
	"os"
	"slices"

	"github.com/git-town/git-town/v14/src/cli/dialog"
	"github.com/git-town/git-town/v14/src/cli/dialog/components"
	"github.com/git-town/git-town/v14/src/cli/flags"
	"github.com/git-town/git-town/v14/src/cmd/cmdhelpers"
	"github.com/git-town/git-town/v14/src/config"
	"github.com/git-town/git-town/v14/src/config/configdomain"
	"github.com/git-town/git-town/v14/src/config/configfile"
	"github.com/git-town/git-town/v14/src/execute"
	"github.com/git-town/git-town/v14/src/git"
	"github.com/git-town/git-town/v14/src/git/gitdomain"
	. "github.com/git-town/git-town/v14/src/gohacks/prelude"
	"github.com/git-town/git-town/v14/src/hosting"
	"github.com/git-town/git-town/v14/src/undo/undoconfig"
	configInterpreter "github.com/git-town/git-town/v14/src/vm/interpreter/config"
	"github.com/spf13/cobra"
)

const setupConfigDesc = "Prompts to setup your Git Town configuration"

func SetupCommand() *cobra.Command {
	addVerboseFlag, readVerboseFlag := flags.Verbose()
	cmd := cobra.Command{
		Use:   "setup",
		Args:  cobra.NoArgs,
		Short: setupConfigDesc,
		Long:  cmdhelpers.Long(setupConfigDesc),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeConfigSetup(readVerboseFlag(cmd))
		},
	}
	addVerboseFlag(&cmd)
	return &cmd
}

// the config settings to be used if the user accepts all default options
func defaultUserInput() userInput {
	return userInput{
		config:        configdomain.DefaultConfig(),
		configStorage: dialog.ConfigStorageOptionFile,
	}
}

func executeConfigSetup(verbose bool) error {
	repo, err := execute.OpenRepo(execute.OpenRepoArgs{
		DryRun:           false,
		OmitBranchNames:  true,
		PrintCommands:    true,
		ValidateGitRepo:  true,
		ValidateIsOnline: false,
		Verbose:          verbose,
	})
	if err != nil {
		return err
	}
	data, exit, err := loadSetupData(repo, verbose)
	if err != nil || exit {
		return err
	}
	aborted, err := enterData(repo.UnvalidatedConfig, repo.Git, repo.Backend, &data)
	if err != nil || aborted {
		return err
	}
	err = saveAll(data.userInput, repo.UnvalidatedConfig, repo.Git, repo.Frontend)
	if err != nil {
		return err
	}
	return configInterpreter.Finished(configInterpreter.FinishedArgs{
		Backend:             repo.Backend,
		BeginConfigSnapshot: repo.ConfigSnapshot,
		Command:             "setup",
		CommandsCounter:     repo.CommandsCounter,
		EndConfigSnapshot:   undoconfig.EmptyConfigSnapshot(),
		FinalMessages:       repo.FinalMessages,
		RootDir:             repo.RootDir,
		Verbose:             verbose,
	})
}

type setupData struct {
	config        config.UnvalidatedConfig
	dialogInputs  components.TestInputs
	hasConfigFile bool
	localBranches gitdomain.BranchInfos
	userInput     userInput
}

func emptySetupData() setupData {
	return setupData{} //exhaustruct:ignore
}

type userInput struct {
	config        configdomain.UnvalidatedConfig
	configStorage dialog.ConfigStorageOption
}

func determineHostingPlatform(config config.UnvalidatedConfig, userChoice Option[configdomain.HostingPlatform]) Option[configdomain.HostingPlatform] {
	if userChoice.IsSome() {
		return userChoice
	}
	if originURL, hasOriginURL := config.OriginURL().Get(); hasOriginURL {
		return hosting.Detect(originURL, userChoice)
	}
	return None[configdomain.HostingPlatform]()
}

func enterData(config config.UnvalidatedConfig, gitCommands git.Commands, backend gitdomain.RunnerQuerier, data *setupData) (aborted bool, err error) {
	aborted, err = dialog.Welcome(data.dialogInputs.Next())
	if err != nil || aborted {
		return aborted, err
	}
	data.userInput.config.Aliases, aborted, err = dialog.Aliases(configdomain.AllAliasableCommands(), config.Config.Aliases, data.dialogInputs.Next())
	if err != nil || aborted {
		return aborted, err
	}
	existingMainBranch := config.Config.MainBranch
	if existingMainBranch.IsNone() {
		existingMainBranch = gitCommands.DefaultBranch(backend)
	}
	if existingMainBranch.IsNone() {
		existingMainBranch = gitCommands.OriginHead(backend)
	}
	mainBranch, aborted, err := dialog.MainBranch(data.localBranches.Names(), existingMainBranch, data.dialogInputs.Next())
	if err != nil || aborted {
		return aborted, err
	}
	data.userInput.config.MainBranch = Some(mainBranch)
	data.userInput.config.PerennialBranches, aborted, err = dialog.PerennialBranches(data.localBranches.Names(), config.Config.PerennialBranches, mainBranch, data.dialogInputs.Next())
	if err != nil || aborted {
		return aborted, err
	}
	data.userInput.config.PerennialRegex, aborted, err = dialog.PerennialRegex(config.Config.PerennialRegex, data.dialogInputs.Next())
	if err != nil || aborted {
		return aborted, err
	}
	data.userInput.config.HostingPlatform, aborted, err = dialog.HostingPlatform(config.Config.HostingPlatform, data.dialogInputs.Next())
	if err != nil || aborted {
		return aborted, err
	}
	if platform, has := determineHostingPlatform(config, data.userInput.config.HostingPlatform).Get(); has {
		switch platform {
		case configdomain.HostingPlatformBitbucket:
			// BitBucket API isn't supported yet
		case configdomain.HostingPlatformGitea:
			data.userInput.config.GiteaToken, aborted, err = dialog.GiteaToken(config.Config.GiteaToken, data.dialogInputs.Next())
			if err != nil || aborted {
				return aborted, err
			}
		case configdomain.HostingPlatformGitHub:
			data.userInput.config.GitHubToken, aborted, err = dialog.GitHubToken(config.Config.GitHubToken, data.dialogInputs.Next())
			if err != nil || aborted {
				return aborted, err
			}
		case configdomain.HostingPlatformGitLab:
			data.userInput.config.GitLabToken, aborted, err = dialog.GitLabToken(config.Config.GitLabToken, data.dialogInputs.Next())
			if err != nil || aborted {
				return aborted, err
			}
		}
	}
	data.userInput.config.HostingOriginHostname, aborted, err = dialog.OriginHostname(config.Config.HostingOriginHostname, data.dialogInputs.Next())
	if err != nil || aborted {
		return aborted, err
	}
	data.userInput.config.SyncFeatureStrategy, aborted, err = dialog.SyncFeatureStrategy(config.Config.SyncFeatureStrategy, data.dialogInputs.Next())
	if err != nil || aborted {
		return aborted, err
	}
	data.userInput.config.SyncPerennialStrategy, aborted, err = dialog.SyncPerennialStrategy(config.Config.SyncPerennialStrategy, data.dialogInputs.Next())
	if err != nil || aborted {
		return aborted, err
	}
	data.userInput.config.SyncUpstream, aborted, err = dialog.SyncUpstream(config.Config.SyncUpstream, data.dialogInputs.Next())
	if err != nil || aborted {
		return aborted, err
	}
	data.userInput.config.PushNewBranches, aborted, err = dialog.PushNewBranches(config.Config.PushNewBranches, data.dialogInputs.Next())
	if err != nil || aborted {
		return aborted, err
	}
	data.userInput.config.PushHook, aborted, err = dialog.PushHook(config.Config.PushHook, data.dialogInputs.Next())
	if err != nil || aborted {
		return aborted, err
	}
	data.userInput.config.SyncBeforeShip, aborted, err = dialog.SyncBeforeShip(config.Config.SyncBeforeShip, data.dialogInputs.Next())
	if err != nil || aborted {
		return aborted, err
	}
	data.userInput.config.ShipDeleteTrackingBranch, aborted, err = dialog.ShipDeleteTrackingBranch(config.Config.ShipDeleteTrackingBranch, data.dialogInputs.Next())
	if err != nil || aborted {
		return aborted, err
	}
	data.userInput.configStorage, aborted, err = dialog.ConfigStorage(data.hasConfigFile, data.dialogInputs.Next())
	if err != nil || aborted {
		return aborted, err
	}
	return false, nil
}

func loadSetupData(repo execute.OpenRepoResult, verbose bool) (setupData, bool, error) {
	dialogTestInputs := components.LoadTestInputs(os.Environ())
	repoStatus, err := repo.Git.RepoStatus(repo.Backend)
	if err != nil {
		return emptySetupData(), false, err
	}
	branchesSnapshot, _, exit, err := execute.LoadRepoSnapshot(execute.LoadRepoSnapshotArgs{
		Backend:               repo.Backend,
		CommandsCounter:       repo.CommandsCounter,
		ConfigSnapshot:        repo.ConfigSnapshot,
		DialogTestInputs:      dialogTestInputs,
		Fetch:                 false,
		FinalMessages:         repo.FinalMessages,
		Frontend:              repo.Frontend,
		Git:                   repo.Git,
		HandleUnfinishedState: true,
		Repo:                  repo,
		RepoStatus:            repoStatus,
		RootDir:               repo.RootDir,
		UnvalidatedConfig:     repo.UnvalidatedConfig,
		ValidateNoOpenChanges: false,
		Verbose:               verbose,
	})
	return setupData{
		config:        repo.UnvalidatedConfig,
		dialogInputs:  dialogTestInputs,
		hasConfigFile: repo.UnvalidatedConfig.ConfigFile.IsSome(),
		localBranches: branchesSnapshot.Branches,
		userInput:     defaultUserInput(),
	}, exit, err
}

func saveAll(userInput userInput, oldConfig config.UnvalidatedConfig, gitCommands git.Commands, frontend gitdomain.Runner) error {
	err := saveAliases(oldConfig.Config.Aliases, userInput.config.Aliases, gitCommands, frontend)
	if err != nil {
		return err
	}
	err = saveGiteaToken(oldConfig.Config.GiteaToken, userInput.config.GiteaToken, gitCommands, frontend)
	if err != nil {
		return err
	}
	err = saveGitHubToken(oldConfig.Config.GitHubToken, userInput.config.GitHubToken, gitCommands, frontend)
	if err != nil {
		return err
	}
	err = saveGitLabToken(oldConfig.Config.GitLabToken, userInput.config.GitLabToken, gitCommands, frontend)
	if err != nil {
		return err
	}
	switch userInput.configStorage {
	case dialog.ConfigStorageOptionFile:
		return saveToFile(userInput, oldConfig)
	case dialog.ConfigStorageOptionGit:
		return saveToGit(userInput, oldConfig, gitCommands, frontend)
	}
	panic("unknown configStorage: " + userInput.configStorage)
}

func saveToGit(userInput userInput, oldConfig config.UnvalidatedConfig, gitCommands git.Commands, frontend gitdomain.Runner) error {
	fc := execute.FailureCollector{}
	fc.Check(saveHostingPlatform(oldConfig.Config.HostingPlatform, userInput.config.HostingPlatform, gitCommands, frontend))
	fc.Check(saveOriginHostname(oldConfig.Config.HostingOriginHostname, userInput.config.HostingOriginHostname, gitCommands, frontend))
	fc.Check(saveMainBranch(oldConfig.Config.MainBranch, userInput.config.MainBranch.GetOrPanic(), oldConfig))
	fc.Check(savePerennialBranches(oldConfig.Config.PerennialBranches, userInput.config.PerennialBranches, oldConfig))
	fc.Check(savePerennialRegex(oldConfig.Config.PerennialRegex, userInput.config.PerennialRegex, oldConfig))
	fc.Check(savePushHook(oldConfig.Config.PushHook, userInput.config.PushHook, oldConfig))
	fc.Check(savePushNewBranches(oldConfig.Config.PushNewBranches, userInput.config.PushNewBranches, oldConfig))
	fc.Check(saveShipDeleteTrackingBranch(oldConfig.Config.ShipDeleteTrackingBranch, userInput.config.ShipDeleteTrackingBranch, oldConfig))
	fc.Check(saveSyncFeatureStrategy(oldConfig.Config.SyncFeatureStrategy, userInput.config.SyncFeatureStrategy, oldConfig))
	fc.Check(saveSyncPerennialStrategy(oldConfig.Config.SyncPerennialStrategy, userInput.config.SyncPerennialStrategy, oldConfig))
	fc.Check(saveSyncUpstream(oldConfig.Config.SyncUpstream, userInput.config.SyncUpstream, oldConfig))
	fc.Check(saveSyncBeforeShip(oldConfig.Config.SyncBeforeShip, userInput.config.SyncBeforeShip, oldConfig))
	return fc.Err
}

func saveAliases(oldAliases, newAliases configdomain.Aliases, gitCommands git.Commands, frontend gitdomain.Runner) (err error) {
	for _, aliasableCommand := range configdomain.AllAliasableCommands() {
		oldAlias, hasOld := oldAliases[aliasableCommand]
		newAlias, hasNew := newAliases[aliasableCommand]
		switch {
		case hasOld && !hasNew:
			err = gitCommands.RemoveGitAlias(frontend, aliasableCommand)
		case newAlias != oldAlias:
			err = gitCommands.SetGitAlias(frontend, aliasableCommand)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func saveGiteaToken(oldToken, newToken Option[configdomain.GiteaToken], gitCommands git.Commands, frontend gitdomain.Runner) error {
	if newToken == oldToken {
		return nil
	}
	if value, has := newToken.Get(); has {
		return gitCommands.SetGiteaToken(frontend, value)
	}
	return gitCommands.RemoveGiteaToken(frontend)
}

func saveGitHubToken(oldToken, newToken Option[configdomain.GitHubToken], gitCommands git.Commands, frontend gitdomain.Runner) error {
	if newToken == oldToken {
		return nil
	}
	if value, has := newToken.Get(); has {
		return gitCommands.SetGitHubToken(frontend, value)
	}
	return gitCommands.RemoveGitHubToken(frontend)
}

func saveGitLabToken(oldToken, newToken Option[configdomain.GitLabToken], gitCommands git.Commands, frontend gitdomain.Runner) error {
	if newToken == oldToken {
		return nil
	}
	if value, has := newToken.Get(); has {
		return gitCommands.SetGitLabToken(frontend, value)
	}
	return gitCommands.RemoveGitLabToken(frontend)
}

func saveHostingPlatform(oldHostingPlatform, newHostingPlatform Option[configdomain.HostingPlatform], gitCommands git.Commands, frontend gitdomain.Runner) (err error) {
	oldValue, oldHas := oldHostingPlatform.Get()
	newValue, newHas := newHostingPlatform.Get()
	if !oldHas && !newHas {
		return nil
	}
	if oldValue == newValue {
		return nil
	}
	if newHas {
		return gitCommands.SetHostingPlatform(frontend, newValue)
	}
	return gitCommands.DeleteHostingPlatform(frontend)
}

func saveMainBranch(oldValue Option[gitdomain.LocalBranchName], newValue gitdomain.LocalBranchName, config config.UnvalidatedConfig) error {
	if Some(newValue) == oldValue {
		return nil
	}
	return config.SetMainBranch(newValue)
}

func saveOriginHostname(oldValue, newValue Option[configdomain.HostingOriginHostname], gitCommands git.Commands, frontend gitdomain.Runner) error {
	if newValue == oldValue {
		return nil
	}
	if value, has := newValue.Get(); has {
		return gitCommands.SetOriginHostname(frontend, value)
	}
	return gitCommands.DeleteOriginHostname(frontend)
}

func savePerennialBranches(oldValue, newValue gitdomain.LocalBranchNames, config config.UnvalidatedConfig) error {
	if slices.Compare(oldValue, newValue) != 0 || config.LocalGitConfig.PerennialBranches == nil {
		return config.SetPerennialBranches(newValue)
	}
	return nil
}

func savePerennialRegex(oldValue, newValue Option[configdomain.PerennialRegex], config config.UnvalidatedConfig) error {
	if newValue == oldValue {
		return nil
	}
	if value, has := newValue.Get(); has {
		return config.SetPerennialRegexLocally(value)
	}
	config.RemovePerennialRegex()
	return nil
}

func savePushHook(oldValue, newValue configdomain.PushHook, config config.UnvalidatedConfig) error {
	if newValue == oldValue {
		return nil
	}
	return config.SetPushHookLocally(newValue)
}

func savePushNewBranches(oldValue, newValue configdomain.PushNewBranches, config config.UnvalidatedConfig) error {
	if newValue == oldValue {
		return nil
	}
	return config.SetPushNewBranches(newValue, false)
}

func saveShipDeleteTrackingBranch(oldValue, newValue configdomain.ShipDeleteTrackingBranch, config config.UnvalidatedConfig) error {
	if newValue == oldValue {
		return nil
	}
	return config.SetShipDeleteTrackingBranch(newValue, false)
}

func saveSyncFeatureStrategy(oldValue, newValue configdomain.SyncFeatureStrategy, config config.UnvalidatedConfig) error {
	if newValue == oldValue {
		return nil
	}
	return config.SetSyncFeatureStrategy(newValue)
}

func saveSyncPerennialStrategy(oldValue, newValue configdomain.SyncPerennialStrategy, config config.UnvalidatedConfig) error {
	if newValue == oldValue {
		return nil
	}
	return config.SetSyncPerennialStrategy(newValue)
}

func saveSyncUpstream(oldValue, newValue configdomain.SyncUpstream, config config.UnvalidatedConfig) error {
	if newValue == oldValue {
		return nil
	}
	return config.SetSyncUpstream(newValue, false)
}

func saveSyncBeforeShip(oldValue, newValue configdomain.SyncBeforeShip, config config.UnvalidatedConfig) error {
	if newValue == oldValue {
		return nil
	}
	return config.SetSyncBeforeShip(newValue, false)
}

func saveToFile(userInput userInput, config config.UnvalidatedConfig) error {
	err := configfile.Save(&userInput.config)
	if err != nil {
		return err
	}
	config.RemoveMainBranch()
	config.RemovePerennialBranches()
	config.RemovePerennialRegex()
	config.RemovePushNewBranches()
	config.RemovePushHook()
	config.RemoveSyncBeforeShip()
	config.RemoveShipDeleteTrackingBranch()
	config.RemoveSyncFeatureStrategy()
	config.RemoveSyncPerennialStrategy()
	config.RemoveSyncUpstream()
	return nil
}
