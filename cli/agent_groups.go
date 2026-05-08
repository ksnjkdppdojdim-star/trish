package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"trish/core"
)

func runGroup(client *core.Client, args []string) int {
	if len(args) == 0 {
		printGroupHelp()
		return 1
	}

	switch args[0] {
	case "list":
		return runGroupList(client)
	case "create":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, yellow("usage: trish group create <name>"))
			return 1
		}
		return printResult(client.CreateGroup(args[1]))
	case "delete", "remove":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, yellow("usage: trish group delete <name>"))
			return 1
		}
		return printResult(client.DeleteGroup(args[1]))
	case "add":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, yellow("usage: trish group add <name> <agent-id>"))
			return 1
		}
		return printResult(client.AddAgentToGroup(args[2], args[1]))
	case "remove-agent":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, yellow("usage: trish group remove-agent <name> <agent-id>"))
			return 1
		}
		return printResult(client.RemoveAgentFromGroup(args[2], args[1]))
	case "exec":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, yellow("usage: trish group exec <name> <command> [args...]"))
			return 1
		}
		return runExecGroup(client, args[1], args[2], args[3:])
	case "help", "-h", "--help":
		printGroupHelp()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "%s %s\n", red("unknown group command:"), args[0])
		printGroupHelp()
		return 1
	}
}

func runTag(client *core.Client, args []string) int {
	if len(args) < 2 {
		printTagHelp()
		return 1
	}

	agentID := args[0]
	action := args[1]
	values := args[2:]
	switch action {
	case "list":
		agent, _, err := client.GetAgent(agentID)
		if err != nil {
			fmt.Fprintln(os.Stderr, red(err.Error()))
			return 1
		}
		if len(agent.Tags) == 0 {
			fmt.Println(dim("No tags"))
			return 0
		}
		fmt.Println(strings.Join(agent.Tags, ", "))
		return 0
	case "set":
		return printResult(client.SetAgentTags(agentID, values))
	case "add":
		if len(values) == 0 {
			fmt.Fprintln(os.Stderr, yellow("usage: trish tag <agent-id> add <tag...>"))
			return 1
		}
		return printResult(client.AddAgentTags(agentID, values))
	case "remove":
		if len(values) == 0 {
			fmt.Fprintln(os.Stderr, yellow("usage: trish tag <agent-id> remove <tag...>"))
			return 1
		}
		return printResult(client.RemoveAgentTags(agentID, values))
	case "help", "-h", "--help":
		printTagHelp()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "%s %s\n", red("unknown tag command:"), action)
		printTagHelp()
		return 1
	}
}

func runGroupList(client *core.Client) int {
	groups, err := client.ListGroups()
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}
	agents, err := client.ListAgents()
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}
	if len(groups) == 0 {
		fmt.Println(dim("No groups"))
		return 0
	}
	rows := make([][]string, 0, len(groups))
	for _, group := range groups {
		count := 0
		online := 0
		for _, agent := range agents {
			if containsString(agent.Groups, group) {
				count++
				if agent.Connected {
					online++
				}
			}
		}
		rows = append(rows, []string{cyan(group), fmt.Sprintf("%d", count), fmt.Sprintf("%d", online)})
	}
	printTable([]string{"Group", "Agents", "Online"}, rows)
	return 0
}

func runExecAll(client *core.Client, command string, args []string) int {
	agents, err := client.ListAgents()
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}
	ids := []string{}
	for _, agent := range agents {
		if agent.Connected {
			ids = append(ids, agent.ID)
		}
	}
	return runExecMany(client, ids, command, args)
}

func runExecGroup(client *core.Client, group string, command string, args []string) int {
	agents, err := client.ListAgents()
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}
	ids := []string{}
	for _, agent := range agents {
		if agent.Connected && containsString(agent.Groups, group) {
			ids = append(ids, agent.ID)
		}
	}
	if len(ids) == 0 {
		fmt.Println(yellow("No online agents in group " + group))
		return 1
	}
	return runExecMany(client, ids, command, args)
}

func runExecMany(client *core.Client, agentIDs []string, command string, args []string) int {
	sort.Strings(agentIDs)
	if len(agentIDs) == 0 {
		fmt.Println(yellow("No online agents matched"))
		return 1
	}

	type execResult struct {
		agentID string
		result  string
		err     error
	}

	results := make(chan execResult, len(agentIDs))
	var wg sync.WaitGroup
	for _, agentID := range agentIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			result, err := client.ExecuteOnAgent(id, command, args)
			results <- execResult{agentID: id, result: result, err: err}
		}(agentID)
	}
	wg.Wait()
	close(results)

	exitCode := 0
	for result := range results {
		fmt.Println(bold(cyan(result.agentID)))
		if result.err != nil {
			fmt.Println(red(result.err.Error()))
			exitCode = 1
		} else {
			fmt.Print(result.result)
			if !strings.HasSuffix(result.result, "\n") {
				fmt.Println()
			}
		}
		fmt.Println()
	}
	return exitCode
}

func printResult(result string, err error) int {
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}
	fmt.Println(green(result))
	return 0
}

func printGroupHelp() {
	fmt.Println(bold("Group commands"))
	fmt.Println("  " + cyan("trish group list"))
	fmt.Println("  " + cyan("trish group create") + " <name>")
	fmt.Println("  " + cyan("trish group delete") + " <name>")
	fmt.Println("  " + cyan("trish group add") + " <name> <agent-id>")
	fmt.Println("  " + cyan("trish group remove-agent") + " <name> <agent-id>")
	fmt.Println("  " + cyan("trish group exec") + " <name> <command> [args...]")
}

func printTagHelp() {
	fmt.Println(bold("Tag commands"))
	fmt.Println("  " + cyan("trish tag") + " <agent-id> list")
	fmt.Println("  " + cyan("trish tag") + " <agent-id> set <tag...>")
	fmt.Println("  " + cyan("trish tag") + " <agent-id> add <tag...>")
	fmt.Println("  " + cyan("trish tag") + " <agent-id> remove <tag...>")
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
