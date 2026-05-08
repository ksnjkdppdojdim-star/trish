package cli

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"trish/buildcfg"
	"trish/core"
)

// Run execute la CLI Trish.
func Run(args []string) int {
	serverAddr, serverPort, remaining, err := parseCommonFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		printHelp()
		return 1
	}

	client := core.NewClient(serverAddr, serverPort)
	if len(remaining) == 0 {
		return runShell(client)
	}

	switch remaining[0] {
	case "list":
		return runList(client)
	case "info":
		if len(remaining) < 2 {
			fmt.Fprintln(os.Stderr, "usage: trish info <agent-id>")
			return 1
		}
		return runInfo(client, remaining[1])
	case "exec", "cmd":
		if len(remaining) < 3 {
			fmt.Fprintln(os.Stderr, "usage: trish exec <agent-id> <command> [args...]")
			return 1
		}
		return runExec(client, remaining[1], remaining[2], remaining[3:])
	case "ping":
		if len(remaining) < 2 {
			fmt.Fprintln(os.Stderr, "usage: trish ping <agent-id>")
			return 1
		}
		return runPing(client, remaining[1])
	case "agent":
		if len(remaining) < 3 {
			fmt.Fprintln(os.Stderr, "usage: trish agent <freeze|unfreeze|stop|restart> <agent-id>")
			return 1
		}
		return runAgentControl(client, remaining[1], remaining[2])
	case "plugin":
		return runPlugin(client, remaining[1:])
	case "shell":
		return runShell(client)
	case "start":
		if len(remaining) >= 2 && remaining[1] == "gui" {
			return runGUI(client, remaining[2:])
		}
		fmt.Fprintln(os.Stderr, "usage: trish start gui")
		return 1
	case "help", "-h", "--help":
		printHelp()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", remaining[0])
		printHelp()
		return 1
	}
}

func runList(client *core.Client) int {
	agents, err := client.ListAgents()
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}

	if len(agents) == 0 {
		fmt.Println(dim("No agents registered"))
		return 0
	}

	sort.Slice(agents, func(i, j int) bool {
		return agents[i].ID < agents[j].ID
	})

	rows := make([][]string, 0, len(agents))
	for _, agent := range agents {
		rows = append(rows, []string{
			cyan(agent.ID),
			agent.Hostname,
			fmt.Sprintf("%s:%d", agent.IPAddress, agent.Port),
			yesNo(agent.Connected),
			statusLabel(agent.Status, agent.Connected),
		})
	}
	printTable([]string{"Agent", "Hostname", "Address", "Connected", "Status"}, rows)

	return 0
}

func runInfo(client *core.Client, agentID string) int {
	agent, commands, err := client.GetAgent(agentID)
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}

	fmt.Println(bold(cyan(agent.ID)))
	printTable([]string{"Field", "Value"}, [][]string{
		{"Hostname", agent.Hostname},
		{"Address", fmt.Sprintf("%s:%d", agent.IPAddress, agent.Port)},
		{"Connected", yesNo(agent.Connected)},
		{"Status", statusLabel(agent.Status, agent.Connected)},
		{"Last Seen", agent.LastSeen.Format("2006-01-02 15:04:05")},
	})

	if len(commands) == 0 {
		fmt.Println(dim("Commands: none"))
		return 0
	}

	sort.Strings(commands)
	fmt.Println()
	fmt.Println(bold("Commands"))
	for _, command := range commands {
		fmt.Printf("  %s\n", cyan(command))
	}

	return 0
}

func runExec(client *core.Client, agentID string, command string, args []string) int {
	result, err := client.ExecuteOnAgent(agentID, command, args)
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}

	fmt.Println(result)
	return 0
}

func runPing(client *core.Client, agentID string) int {
	result, err := client.PingAgent(agentID)
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}

	fmt.Println(green(result))
	return 0
}

func runAgentControl(client *core.Client, control string, agentID string) int {
	switch control {
	case "freeze", "unfreeze", "stop", "restart":
	default:
		fmt.Fprintf(os.Stderr, "%s %s\n", red("unsupported agent control:"), control)
		return 1
	}

	result, err := client.ControlAgent(agentID, control)
	if err != nil {
		fmt.Fprintln(os.Stderr, red(err.Error()))
		return 1
	}

	fmt.Println(green(result))
	return 0
}

func runShell(client *core.Client) int {
	printBanner()
	printCommandHint()
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	activeAgent := ""
	for {
		fmt.Printf("%s> ", shellPrompt(activeAgent))
		if !scanner.Scan() {
			return 0
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if line == "exit" || line == "quit" {
			return 0
		}

		if line == "help" {
			printCommandHint()
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		switch fields[0] {
		case "use", "select", "switch":
			if len(fields) != 2 {
				fmt.Println(yellow("usage: use <agent-id>"))
				continue
			}
			activeAgent = fields[1]
			fmt.Printf("%s %s\n", green("Active agent:"), cyan(activeAgent))
			continue
		case "clear":
			activeAgent = ""
			fmt.Println(dim("Active agent cleared"))
			continue
		case "current":
			if activeAgent == "" {
				fmt.Println(yellow("No active agent"))
			} else {
				fmt.Printf("%s %s\n", green("Active agent:"), cyan(activeAgent))
			}
			continue
		}

		resolved, err := resolveShellCommand(fields, activeAgent)
		if err != nil {
			fmt.Fprintln(os.Stderr, red(err.Error()))
			continue
		}

		code := Run(append([]string{
			fmt.Sprintf("-server=%s", client.ServerAddr),
			fmt.Sprintf("-port=%d", client.ServerPort),
		}, resolved...))
		if code != 0 {
			fmt.Printf("%s %d\n", red("command failed with exit code"), code)
		}
	}
}

func resolveShellCommand(fields []string, activeAgent string) ([]string, error) {
	if len(fields) == 0 {
		return nil, nil
	}

	switch fields[0] {
	case "list", "help", "exit", "quit", "shell", "start", "plugin":
		return fields, nil
	case "info", "ping":
		if len(fields) == 1 {
			if activeAgent == "" {
				return nil, fmt.Errorf("no active agent selected")
			}
			return []string{fields[0], activeAgent}, nil
		}
		return fields, nil
	case "exec", "cmd":
		if activeAgent != "" {
			if len(fields) < 2 {
				return nil, fmt.Errorf("usage: exec <command> [args...]")
			}
			if len(fields) >= 3 && strings.EqualFold(fields[1], activeAgent) {
				return fields, nil
			}
			return append([]string{"exec", activeAgent}, fields[1:]...), nil
		}
		if len(fields) >= 3 {
			return fields, nil
		}
		return nil, fmt.Errorf("usage: exec <agent-id> <command> [args...]")
	case "agent":
		if len(fields) >= 3 {
			return fields, nil
		}
		if activeAgent == "" || len(fields) != 2 {
			return nil, fmt.Errorf("usage: agent <freeze|unfreeze|stop|restart> <agent-id>")
		}
		return []string{"agent", fields[1], activeAgent}, nil
	default:
		if activeAgent == "" {
			return nil, fmt.Errorf("unknown command %q and no active agent selected", fields[0])
		}
		return append([]string{"exec", activeAgent}, fields...), nil
	}
}

func shellPrompt(activeAgent string) string {
	if activeAgent == "" {
		return cyan("trish")
	}
	return cyan("trish") + "[" + green(activeAgent) + "]"
}

func parseCommonFlags(args []string) (string, int, []string, error) {
	serverAddr := buildcfg.DefaultServerAddr
	serverPort, err := strconv.Atoi(strings.TrimSpace(buildcfg.DefaultServerPort))
	if err != nil {
		serverPort = 9999
	}
	remaining := make([]string, 0, len(args))

	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "-server="):
			serverAddr = strings.TrimPrefix(arg, "-server=")
		case strings.HasPrefix(arg, "--server="):
			serverAddr = strings.TrimPrefix(arg, "--server=")
		case strings.HasPrefix(arg, "-port="):
			_, err := fmt.Sscanf(strings.TrimPrefix(arg, "-port="), "%d", &serverPort)
			if err != nil {
				return "", 0, nil, fmt.Errorf("invalid port flag: %s", arg)
			}
		case strings.HasPrefix(arg, "--port="):
			_, err := fmt.Sscanf(strings.TrimPrefix(arg, "--port="), "%d", &serverPort)
			if err != nil {
				return "", 0, nil, fmt.Errorf("invalid port flag: %s", arg)
			}
		case strings.HasPrefix(arg, "-admin-secret="):
			buildcfg.DefaultAdminSecret = strings.TrimPrefix(arg, "-admin-secret=")
		case strings.HasPrefix(arg, "--admin-secret="):
			buildcfg.DefaultAdminSecret = strings.TrimPrefix(arg, "--admin-secret=")
		default:
			remaining = append(remaining, arg)
		}
	}

	return serverAddr, serverPort, remaining, nil
}

func printHelp() {
	printBanner()
	fmt.Println()
	fmt.Println(bold("Usage"))
	fmt.Println("  " + cyan("trish") + " [-server=HOST] [-port=9999] list")
	fmt.Println("  " + cyan("trish") + " [-server=HOST] [-port=9999] info <agent-id>")
	fmt.Println("  " + cyan("trish") + " [-server=HOST] [-port=9999] ping <agent-id>")
	fmt.Println("  " + cyan("trish") + " [-server=HOST] [-port=9999] exec <agent-id> <command> [args...]")
	fmt.Println("  " + cyan("trish") + " [-server=HOST] [-port=9999] agent <freeze|unfreeze|stop|restart> <agent-id>")
	fmt.Println("  " + cyan("trish") + " [-server=HOST] [-port=9999] plugin <install|update|list|status|remove> ...")
	fmt.Println("  " + cyan("trish") + " [-server=HOST] [-port=9999] shell")
	fmt.Println("  " + cyan("trish") + " [-server=HOST] [-port=9999] start gui")
}

func defaultStatus(status string) string {
	if status == "" {
		return "unknown"
	}
	return status
}
