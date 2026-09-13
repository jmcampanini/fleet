// Package config loads Fleet's single TOML inventory and scalar overrides.
package config

import (
	"fmt"

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

// Load applies defaults, one required file, environment, then root flags.
func Load(path string, flags *pflag.FlagSet) (Config, configloader.LoadReport, inventory.Inventory, error) {
	if path == "" {
		helper, err := configloader.NewFileHelper("fleet", "fleet.toml")
		if err != nil {
			return Config{}, configloader.LoadReport{}, inventory.Inventory{}, err
		}
		paths := helper.XDGConfigFile()
		if len(paths) == 0 {
			return Config{}, configloader.LoadReport{}, inventory.Inventory{}, fmt.Errorf("cannot discover configuration; supply --config PATH")
		}
		path = paths[0]
	}
	fileLoader, err := configloader.NewRequiredFileLoader[Config](path)
	if err != nil {
		return Config{}, configloader.LoadReport{}, inventory.Inventory{}, fmt.Errorf("load config %q; create this TOML file or use --config PATH: %w", path, err)
	}
	envLoader, err := configloader.NewEnvironmentLoader[Config]("fleet", configloader.OSEnv())
	if err != nil {
		return Config{}, configloader.LoadReport{}, inventory.Inventory{}, err
	}
	flagLoader, err := pflagloader.NewLoader[Config](flags)
	if err != nil {
		return Config{}, configloader.LoadReport{}, inventory.Inventory{}, err
	}
	cfg, report, err := configloader.Load(defaults(), fileLoader, envLoader, flagLoader)
	if err != nil {
		return Config{}, configloader.LoadReport{}, inventory.Inventory{}, fmt.Errorf("load config %q: %w", path, err)
	}
	if cfg.Issues.Limit <= 0 || cfg.PRs.Limit <= 0 {
		return Config{}, configloader.LoadReport{}, inventory.Inventory{}, fmt.Errorf("issues.limit and prs.limit must be positive integers")
	}

	branches := make(map[string]string, len(cfg.Repos))
	for id, repo := range cfg.Repos {
		branches[id] = repo.Branch
	}
	inv, err := inventory.New(branches, cfg.Groups)
	if err != nil {
		return Config{}, configloader.LoadReport{}, inventory.Inventory{}, err
	}
	return cfg, report, inv, nil
}
