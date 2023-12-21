package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/tiagoposse/terraform-provider-oas-codegen/extension"
)

type GenerateAllCommand struct {
	UI                 cli.Ui
	flagConfigPath     string
	flagRepositoryPath string
}

func (cmd *GenerateAllCommand) Flags() *flag.FlagSet {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	fs.StringVar(&cmd.flagConfigPath, "config", "", "configuration file path")
	fs.StringVar(&cmd.flagRepositoryPath, "repo", "", "path for the repository")

	return fs
}

func (cmd *GenerateAllCommand) Help() string {
	strBuilder := &strings.Builder{}

	longestName := 0
	longestUsage := 0
	cmd.Flags().VisitAll(func(f *flag.Flag) {
		if len(f.Name) > longestName {
			longestName = len(f.Name)
		}
		if len(f.Usage) > longestUsage {
			longestUsage = len(f.Usage)
		}
	})

	strBuilder.WriteString("\nUsage: tfplugingen-framework generate all [<args>]\n\n")
	cmd.Flags().VisitAll(func(f *flag.Flag) {
		if f.DefValue != "" {
			strBuilder.WriteString(fmt.Sprintf("    --%s <ARG> %s%s%s  (default: %q)\n",
				f.Name,
				strings.Repeat(" ", longestName-len(f.Name)+2),
				f.Usage,
				strings.Repeat(" ", longestUsage-len(f.Usage)+2),
				f.DefValue,
			))
		} else {
			strBuilder.WriteString(fmt.Sprintf("    --%s <ARG> %s%s%s\n",
				f.Name,
				strings.Repeat(" ", longestName-len(f.Name)+2),
				f.Usage,
				strings.Repeat(" ", longestUsage-len(f.Usage)+2),
			))
		}
	})
	strBuilder.WriteString("\n")

	return strBuilder.String()
}
func (a *GenerateAllCommand) Synopsis() string {
	return "Terraform Provider OAS code generation"
}

func (cmd *GenerateAllCommand) Run(args []string) int {
	ctx := context.Background()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	fs := cmd.Flags()
	err := fs.Parse(args)
	if err != nil {
		logger.Error("error parsing command flags", "err", err)
		return 1
	}

	err = cmd.runInternal(ctx, logger)
	if err != nil {
		logger.Error("error executing command", "err", err)
		return 1
	}

	return 0
}

func (cmd *GenerateAllCommand) runInternal(ctx context.Context, logger *slog.Logger) error {
	var repopath, confpath string
	if cmd.flagRepositoryPath == "" {
		pwd, _ := os.Getwd()

		for {
			possible_modpath := filepath.Join(pwd, "go.mod")
			if _, err := os.Stat(possible_modpath); err == nil && !os.IsNotExist(err) {
				repopath = pwd
			} else if !os.IsNotExist(err) {
				return err
			}

			if index := strings.LastIndex(pwd, string(filepath.Separator)); index > -1 {
				pwd = pwd[0:index]
			} else {
				break
			}
		}
	} else {
		repopath = cmd.flagRepositoryPath
	}

	content, err := os.ReadFile(filepath.Join(repopath, "go.mod"))
	if err != nil {
		return fmt.Errorf("reading mod file: %w", err)
	}

	re := regexp.MustCompile(`module (.+)`)
	m := re.FindSubmatch(content)

	fmt.Println(cmd.flagConfigPath)
	if cmd.flagConfigPath == "" {
		confpath = filepath.Join(repopath, "generate", "config.yaml")
	} else {
		confpath = cmd.flagConfigPath
	}

	if _, err := os.Stat(confpath); err != nil {
		fmt.Printf("configuration not found: %v\n", err)
	}

	tfext := extension.TerraformExtension{
		PackageName: string(m[1]),
		RepoPath:    repopath,
		ConfigPath:  confpath,
	}

	if err := tfext.Generate(); err != nil {
		return fmt.Errorf("error generating: %v", err)
	}

	return nil
}
