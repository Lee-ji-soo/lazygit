package commands

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-errors/errors"
	gogit "github.com/jesseduffield/go-git/v5"
	"github.com/jesseduffield/lazygit/pkg/commands/oscommands"
	"github.com/jesseduffield/lazygit/pkg/env"
	"github.com/jesseduffield/lazygit/pkg/i18n"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

func getRepoInfo(tr *i18n.TranslationSet) (*gogit.Repository, string, error) {
	if err := navigateToRepoRootDirectory(os.Stat, os.Chdir); err != nil {
		return nil, "", err
	}

	repo, err := setupRepository(gogit.PlainOpen, tr.GitconfigParseErr)
	if err != nil {
		return nil, "", err
	}

	dotGitDir, err := findDotGitDir(os.Stat, ioutil.ReadFile)
	if err != nil {
		return nil, "", err
	}

	return repo, dotGitDir, nil
}

func navigateToRepoRootDirectory(stat func(string) (os.FileInfo, error), chdir func(string) error) error {
	gitDir := env.GetGitDirEnv()
	if gitDir != "" {
		// we've been given the git directory explicitly so no need to navigate to it
		_, err := stat(gitDir)
		if err != nil {
			return utils.WrapError(err)
		}

		return nil
	}

	// we haven't been given the git dir explicitly so we assume it's in the current working directory as `.git/` (or an ancestor directory)

	for {
		_, err := stat(".git")

		if err == nil {
			return nil
		}

		if !os.IsNotExist(err) {
			return utils.WrapError(err)
		}

		if err = chdir(".."); err != nil {
			return utils.WrapError(err)
		}

		currentPath, err := os.Getwd()
		if err != nil {
			return err
		}

		atRoot := currentPath == filepath.Dir(currentPath)
		if atRoot {
			// we should never really land here: the code that creates Git should
			// verify we're in a git directory
			return errors.New("Must open lazygit in a git repository")
		}
	}
}

func setupRepository(openGitRepository func(string) (*gogit.Repository, error), gitConfigParseErrorStr string) (*gogit.Repository, error) {
	unresolvedPath := env.GetGitDirEnv()
	if unresolvedPath == "" {
		var err error
		unresolvedPath, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	path, err := resolvePath(unresolvedPath)
	if err != nil {
		return nil, err
	}

	repository, err := openGitRepository(path)

	if err != nil {
		if strings.Contains(err.Error(), `unquoted '\' must be followed by new line`) {
			return nil, errors.New(gitConfigParseErrorStr)
		}

		return nil, err
	}

	return repository, err
}

// resolvePath takes a path containing a symlink and returns the true path
func resolvePath(path string) (string, error) {
	l, err := os.Lstat(path)
	if err != nil {
		return "", err
	}

	if l.Mode()&os.ModeSymlink == 0 {
		return path, nil
	}

	return filepath.EvalSymlinks(path)
}

func findDotGitDir(stat func(string) (os.FileInfo, error), readFile func(filename string) ([]byte, error)) (string, error) {
	if env.GetGitDirEnv() != "" {
		return env.GetGitDirEnv(), nil
	}

	f, err := stat(".git")
	if err != nil {
		return "", err
	}

	if f.IsDir() {
		return ".git", nil
	}

	fileBytes, err := readFile(".git")
	if err != nil {
		return "", err
	}
	fileContent := string(fileBytes)
	if !strings.HasPrefix(fileContent, "gitdir: ") {
		return "", errors.New(".git is a file which suggests we are in a submodule but the file's contents do not contain a gitdir pointing to the actual .git directory")
	}
	return strings.TrimSpace(strings.TrimPrefix(fileContent, "gitdir: ")), nil
}

func VerifyInGitRepo(osCommand *oscommands.OS) error {
	return osCommand.Run(
		BuildGitCmdObjFromStr("rev-parse --git-dir"),
	)
}