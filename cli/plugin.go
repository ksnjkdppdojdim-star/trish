// Copyright (c) 2026 Jules MAHOUNOU
// Project  : TRISH
// Initiated: 17/04/2026
// Origin   : Benin
// Contact  : jtodjinou@datatechnologies.bj | +229 0159521211
// License  : MIT — see LICENSE file for details

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"trish/core"
)

func runPlugin(client *core.Client, args []string) int {
	if len(args) == 0 {
		printPluginHelp()
		return 1
	}

	switch args[0] {
	case "install":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, yellow("usage: trish plugin install <path-or-git-url>"))
			return 1
		}
		return runPluginInstall(client, args[1])
	case "update":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, yellow("usage: trish plugin update <path-or-git-url|all>"))
			return 1
		}
		if args[1] == "all" {
			return runPluginUpdateAll(client)
		}
		return runPluginInstall(client, args[1])
	case "list":
		return runPluginList(client)
	case "enable":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, yellow("usage: trish plugin enable <name>"))
			return 1
		}
		return runPluginToggle(client, args[1], true)
	case "disable":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, yellow("usage: trish plugin disable <name>"))
			return 1
		}
		return runPluginToggle(client, args[1], false)
	case "versions":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, yellow("usage: trish plugin versions <name>"))
			return 1
		}
		return runPluginVersions(client, args[1])
	case "rollback":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, yellow("usage: trish plugin rollback <name> <version>"))
			return 1
		}
		return runPluginRollback(client, args[1], args[2])
	case "test":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, yellow("usage: trish plugin test <path-or-git-url>"))
			return 1
		}
		return runPluginTest(args[1])
	case "remove", "uninstall":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, yellow("usage: trish plugin remove <name>"))
			return 1
		}
		result, err := client.RemovePlugin(args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, red(err.Error()))
			return 1
		}
		fmt.Println(green(result))
		return 0
	case "status", "info":
		if len(args) > 2 {
			fmt.Fprintln(os.Stderr, yellow("usage: trish plugin status [agent-id]"))
			return 1
		}
		agentID := ""
		if len(args) == 2 {
			agentID = args[1]
		}
		return runPluginStatus(client, agentID)
	case "help", "-h", "--help":
		printPluginHelp()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "%s %s\n", red("unknown plugin command:"), args[0])
		printPluginHelp()
		return 1
	}
}

func runPluginInstall(client *core.Client, source string) int {
	pkg, cleanup, err := packagePluginSource(source)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}

	result, err := client.InstallPlugin(pkg)
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}
	fmt.Println(green(result))
	return 0
}

func runPluginToggle(client *core.Client, name string, enabled bool) int {
	var (
		result string
		err    error
	)
	if enabled {
		result, err = client.EnablePlugin(name)
	} else {
		result, err = client.DisablePlugin(name)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}
	fmt.Println(green(result))
	return 0
}

func runPluginVersions(client *core.Client, name string) int {
	versions, err := client.ListPluginVersions(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}
	rows := make([][]string, 0, len(versions))
	for _, plugin := range versions {
		state := green("active")
		if plugin.Disabled {
			state = dim("stored")
		}
		rows = append(rows, []string{
			plugin.Version,
			state,
			shortChecksum(plugin.ChecksumSHA256),
			strings.Join(plugin.Permissions, ", "),
			plugin.Source,
		})
	}
	printTable([]string{"Version", "State", "Checksum", "Permissions", "Source"}, rows)
	return 0
}

func runPluginRollback(client *core.Client, name string, version string) int {
	result, err := client.RollbackPlugin(name, version)
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}
	fmt.Println(green(result))
	return 0
}

func runPluginTest(source string) int {
	pkg, cleanup, err := packagePluginSource(source)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}

	fmt.Println(green("plugin package is valid"))
	printTable([]string{"Field", "Value"}, [][]string{
		{"Name", cyan(pkg.Manifest.Name)},
		{"Version", pkg.Manifest.Version},
		{"Entry", pkg.Manifest.Entry},
		{"Shell", pkg.Manifest.Shell},
		{"Commands", strings.Join(pkg.Manifest.CommandNames(), ", ")},
		{"Permissions", strings.Join(pkg.Manifest.Permissions, ", ")},
		{"Checksum", pkg.Manifest.ChecksumSHA256},
	})
	return 0
}

func runPluginUpdateAll(client *core.Client) int {
	plugins, err := client.ListPlugins()
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}
	if len(plugins) == 0 {
		fmt.Println(dim("No plugins installed"))
		return 0
	}

	exitCode := 0
	for _, plugin := range plugins {
		if strings.TrimSpace(plugin.Source) == "" {
			fmt.Fprintf(os.Stderr, "%s %s has no source; skipping\n", yellow("plugin"), plugin.Name)
			exitCode = 1
			continue
		}
		fmt.Printf("%s %s %s %s\n", cyan("Updating"), bold(plugin.Name), dim("from"), plugin.Source)
		if code := runPluginInstall(client, plugin.Source); code != 0 {
			exitCode = code
		}
	}
	return exitCode
}

func runPluginList(client *core.Client) int {
	plugins, err := client.ListPlugins()
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}
	if len(plugins) == 0 {
		fmt.Println(dim("No plugins installed"))
		return 0
	}

	rows := make([][]string, 0, len(plugins))
	for _, plugin := range plugins {
		commands := plugin.CommandNames()
		state := green("enabled")
		if plugin.Disabled {
			state = yellow("disabled")
		}
		rows = append(rows, []string{
			cyan(plugin.Name),
			plugin.Version,
			state,
			strings.Join(commands, ", "),
			shortChecksum(plugin.ChecksumSHA256),
			plugin.Source,
		})
	}
	printTable([]string{"Plugin", "Version", "State", "Commands", "Checksum", "Source"}, rows)
	return 0
}

func runPluginStatus(client *core.Client, agentID string) int {
	plugins, err := client.ListPlugins()
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}
	if len(plugins) == 0 {
		fmt.Println(dim("No plugins installed on server"))
	} else {
		fmt.Println(bold("Server plugins"))
		rows := make([][]string, 0, len(plugins))
		for _, plugin := range plugins {
			state := green("enabled")
			if plugin.Disabled {
				state = yellow("disabled")
			}
			rows = append(rows, []string{
				cyan(plugin.Name),
				plugin.Version,
				state,
				strings.Join(plugin.CommandNames(), ", "),
			})
		}
		printTable([]string{"Plugin", "Version", "State", "Commands"}, rows)
	}

	if strings.TrimSpace(agentID) == "" {
		return 0
	}
	agent, commands, err := client.GetAgent(agentID)
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}
	sort.Strings(commands)
	fmt.Printf("\n%s %s %s\n", bold("Agent"), cyan(agent.ID), statusLabel(agent.Status, agent.Connected))
	fmt.Println(bold("Commands"))
	for _, command := range commands {
		fmt.Printf("  %s\n", cyan(command))
	}
	return 0
}

func packagePluginSource(source string) (*core.DynamicPluginPackage, func(), error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, nil, fmt.Errorf("plugin source is required")
	}

	if isGitSource(source) {
		tempDir, err := os.MkdirTemp("", "trish-plugin-*")
		if err != nil {
			return nil, nil, err
		}
		cleanup := func() { _ = os.RemoveAll(tempDir) }
		cmd := exec.Command("git", "clone", "--depth", "1", source, tempDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("git clone failed: %w\n%s", err, strings.TrimSpace(string(output)))
		}
		pkg, err := core.LoadDynamicPluginPackage(tempDir, source)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		return pkg, cleanup, nil
	}

	abs, err := filepath.Abs(source)
	if err != nil {
		return nil, nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("plugin source must be a directory or git URL: %s", source)
	}
	pkg, err := core.LoadDynamicPluginPackage(abs, abs)
	return pkg, nil, err
}

func isGitSource(source string) bool {
	return strings.HasPrefix(source, "https://") ||
		strings.HasPrefix(source, "http://") ||
		strings.HasPrefix(source, "git@") ||
		strings.HasSuffix(source, ".git")
}

func printPluginHelp() {
	fmt.Println(bold("Plugin commands"))
	fmt.Println("  " + cyan("trish plugin install") + " <path-or-git-url>")
	fmt.Println("  " + cyan("trish plugin update") + " <path-or-git-url|all>")
	fmt.Println("  " + cyan("trish plugin test") + " <path-or-git-url>")
	fmt.Println("  " + cyan("trish plugin list"))
	fmt.Println("  " + cyan("trish plugin enable") + " <name>")
	fmt.Println("  " + cyan("trish plugin disable") + " <name>")
	fmt.Println("  " + cyan("trish plugin versions") + " <name>")
	fmt.Println("  " + cyan("trish plugin rollback") + " <name> <version>")
	fmt.Println("  " + cyan("trish plugin status") + " [agent-id]")
	fmt.Println("  " + cyan("trish plugin info") + " [agent-id]")
	fmt.Println("  " + cyan("trish plugin remove") + " <name>")
}

func shortChecksum(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}
