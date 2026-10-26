// Package config loads Fleet's single TOML inventory and scalar overrides.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/jmcampanini/fleet/internal/inventory"
	"github.com/jmcampanini/go-config-loader/configloader"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/spf13/pflag"
)

// Issues holds the total issue limit.
type Issues struct {
	Limit int `toml:"limit" config:"issues-limit" help:"Maximum total issues across selected repositories"`
}

// PRs holds the total pull request limit.
type PRs struct {
	Limit int `toml:"limit" config:"prs-limit" help:"Maximum total pull requests across selected repositories"`
}

// Repo holds the optional settings of one configured repository.
type Repo struct {
	Branch string `toml:"branch"`
}

// Config is the effective application configuration.
type Config struct {
	Groups map[string][]string `toml:"groups"`
	Issues Issues              `toml:"issues"`
	PRs    PRs                 `toml:"prs"`
	Repos  map[string]Repo     `toml:"repos"`
}

func defaults() Config {
	return Config{Issues: Issues{Limit: 15}, PRs: PRs{Limit: 15}}
}

// Loaded is the validated outcome of one configuration load.
type Loaded struct {
	Config    Config
	Inventory inventory.Inventory
	Report    configloader.LoadReport
}

// Load applies defaults, the configuration file, environment, then root
// flags, and validates the inventory before returning. A path the caller
// supplies must load. The discovered user-level file may be absent; when
// something exists at that path it must load too.
func Load(path string, flags *pflag.FlagSet) (Loaded, error) {
	discovered := path == ""
	if discovered {
		helper, err := configloader.NewFileHelper("fleet", "config.toml")
		if err != nil {
			return Loaded{}, err
		}
		paths := helper.XDGConfigFile()
		if len(paths) == 0 {
			return Loaded{}, fmt.Errorf("cannot discover configuration; supply --config PATH")
		}
		path = paths[0]
	}

	var fileLoader configloader.ConfigLoader[Config]
	var err error
	if discovered && absent(path) {
		fileLoader, err = configloader.NewPickLastFileLoader[Config]([]string{path})
	} else {
		fileLoader, err = configloader.NewRequiredFileLoader[Config](path)
	}
	if err != nil {
		return Loaded{}, fmt.Errorf("load config %q: %w", path, err)
	}
	envLoader, err := configloader.NewEnvironmentLoader[Config]("fleet", configloader.OSEnv())
	if err != nil {
		return Loaded{}, err
	}
	flagLoader, err := pflagloader.NewLoader[Config](flags)
	if err != nil {
		return Loaded{}, err
	}
	cfg, report, err := configloader.Load(defaults(), fileLoader, envLoader, flagLoader)
	if err != nil {
		return Loaded{}, fmt.Errorf("load config %q: %w", path, err)
	}
	if cfg.Issues.Limit <= 0 || cfg.PRs.Limit <= 0 {
		return Loaded{}, fmt.Errorf("issues.limit and prs.limit must be positive integers")
	}

	branches := make(map[string]string, len(cfg.Repos))
	for id, repo := range cfg.Repos {
		branches[id] = repo.Branch
	}
	inv, err := inventory.New(branches, cfg.Groups)
	if err != nil {
		return Loaded{}, err
	}
	return Loaded{Config: cfg, Inventory: inv, Report: report}, nil
}

// absent reports whether nothing exists at path.
func absent(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, fs.ErrNotExist)
}
