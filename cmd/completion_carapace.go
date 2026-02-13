package cmd

import (
	"github.com/carapace-sh/carapace"
	"github.com/deca-org/deca/internal/github"
)

func init() {
	// Initialize carapace for all commands
	carapace.Gen(RootCmd)

	// Setup completers for subcommands
	setupAddCompletions()
	setupRemoveCompletions()
	setupUpdateCompletions()
	setupStatusCompletions()
}

// setupAddCompletions sets up completions for the add command
func setupAddCompletions() {
	// Complete repository names (owner/repo format)
	carapace.Gen(AddCmd).PositionalCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			if len(c.Args) == 0 {
				return carapace.ActionValues()
			}

			query := c.Args[0]
			if query == "" {
				return carapace.ActionValues()
			}

			// Use GitHub search
			ghClient := github.NewClient()
			repos, err := ghClient.SearchRepositories(getContext(), query)
			if err != nil {
				return carapace.ActionValues()
			}

			var pairs []string
			for _, r := range repos {
				desc := r.Desc
				if desc != "" && len(desc) > 60 {
					desc = desc[:57] + "..."
				}
				pairs = append(pairs, r.FullName, desc)
			}
			return carapace.ActionValuesDescribed(pairs...)
		}),
	)

	// Complete asset flag with assets from the specified repo
	carapace.Gen(AddCmd).FlagCompletion(carapace.ActionMap{
		"asset": carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			// First get the repo from args
			if len(c.Args) == 0 {
				return carapace.ActionValues()
			}

			repo := c.Args[0]
			owner, repoName, err := github.ParseRepo(repo)
			if err != nil {
				return carapace.ActionValues()
			}

			// Fetch release assets
			ghClient := github.NewClient()
			release, err := ghClient.GetLatestRelease(getContext(), owner, repoName)
			if err != nil {
				return carapace.ActionValues()
			}

			values := make([]string, len(release.Assets))
			for i, a := range release.Assets {
				values[i] = a.Name
			}
			return carapace.ActionValues(values...)
		}),
	})
}

// setupRemoveCompletions sets up completions for the remove command
func setupRemoveCompletions() {
	carapace.Gen(RemoveCmd).PositionalCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			cfg, err := loadConfig()
			if err != nil {
				return carapace.ActionValues()
			}

			values := make([]string, 0, len(cfg.Packages))
			for name := range cfg.Packages {
				values = append(values, name)
			}
			return carapace.ActionValues(values...)
		}),
	)
}

// setupUpdateCompletions sets up completions for the update command
func setupUpdateCompletions() {
	carapace.Gen(UpdateCmd).PositionalCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			cfg, err := loadConfig()
			if err != nil {
				return carapace.ActionValues()
			}

			values := make([]string, 0, len(cfg.Packages))
			for name := range cfg.Packages {
				values = append(values, name)
			}
			return carapace.ActionValues(values...)
		}),
	)
}

// setupStatusCompletions sets up completions for the status command
func setupStatusCompletions() {
	carapace.Gen(StatusCmd).PositionalCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			cfg, err := loadConfig()
			if err != nil {
				return carapace.ActionValues()
			}

			values := make([]string, 0, len(cfg.Packages))
			for name := range cfg.Packages {
				values = append(values, name)
			}
			return carapace.ActionValues(values...)
		}),
	)
}
