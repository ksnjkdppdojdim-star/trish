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
	case "shell":
		return runShell(client)
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
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if len(agents) == 0 {
		fmt.Println("No agents registered")
		return 0
	}

	sort.Slice(agents, func(i, j int) bool {
		return agents[i].ID < agents[j].ID
	})

	for _, agent := range agents {
		fmt.Printf("%s\t%s\t%s:%d\tconnected=%t\tstatus=%s\n", agent.ID, agent.Hostname, agent.IPAddress, agent.Port, agent.Connected, defaultStatus(agent.Status))
	}

	return 0
}

func runInfo(client *core.Client, agentID string) int {
	agent, commands, err := client.GetAgent(agentID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Printf("Agent ID: %s\n", agent.ID)
	fmt.Printf("Hostname: %s\n", agent.Hostname)
	fmt.Printf("Address: %s:%d\n", agent.IPAddress, agent.Port)
	fmt.Printf("Connected: %t\n", agent.Connected)
	fmt.Printf("Status: %s\n", defaultStatus(agent.Status))
	fmt.Printf("Last Seen: %s\n", agent.LastSeen.Format("2006-01-02 15:04:05"))

	if len(commands) == 0 {
		fmt.Println("Commands: none")
		return 0
	}

	sort.Strings(commands)
	fmt.Println("Commands:")
	for _, command := range commands {
		fmt.Printf("  - %s\n", command)
	}

	return 0
}

func runExec(client *core.Client, agentID string, command string, args []string) int {
	result, err := client.ExecuteOnAgent(agentID, command, args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Println(result)
	return 0
}

func runPing(client *core.Client, agentID string) int {
	result, err := client.PingAgent(agentID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Println(result)
	return 0
}

func runAgentControl(client *core.Client, control string, agentID string) int {
	switch control {
	case "freeze", "unfreeze", "stop", "restart":
	default:
		fmt.Fprintf(os.Stderr, "unsupported agent control: %s\n", control)
		return 1
	}

	result, err := client.ControlAgent(agentID, control)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Println(result)
	return 0
}

func runShell(client *core.Client) int {
	fmt.Println("Trish shell")
	fmt.Println("Commands: list, info <agent>, exec <agent> <cmd> [args], help, exit")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("trish> ")
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
			fmt.Println("list | info <agent> | ping <agent> | exec <agent> <cmd> [args] | agent <freeze|unfreeze|stop|restart> <agent> | exit")
			continue
		}

		code := Run(append([]string{
			fmt.Sprintf("-server=%s", client.ServerAddr),
			fmt.Sprintf("-port=%d", client.ServerPort),
		}, strings.Fields(line)...))
		if code != 0 {
			fmt.Printf("command failed with exit code %d\n", code)
		}
	}
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
		default:
			remaining = append(remaining, arg)
		}
	}

	return serverAddr, serverPort, remaining, nil
}

func printHelp() {
	fmt.Println("Trish - Remote PC Management")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  trish [-server=HOST] [-port=9999] list")
	fmt.Println("  trish [-server=HOST] [-port=9999] info <agent-id>")
	fmt.Println("  trish [-server=HOST] [-port=9999] ping <agent-id>")
	fmt.Println("  trish [-server=HOST] [-port=9999] exec <agent-id> <command> [args...]")
	fmt.Println("  trish [-server=HOST] [-port=9999] agent <freeze|unfreeze|stop|restart> <agent-id>")
	fmt.Println("  trish [-server=HOST] [-port=9999] shell")
}

func defaultStatus(status string) string {
	if status == "" {
		return "unknown"
	}
	return status
}
