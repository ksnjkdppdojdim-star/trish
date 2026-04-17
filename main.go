package main

import (
	"fmt"
	"log"
	"os"
	"trish/core"
	"trish/plugins/cd"
	"trish/plugins/dir"
	"trish/plugins/ipconfig"

	"github.com/spf13/cobra"
)

var (
	client *core.Client
	agent  *core.Agent
)

func init() {
	// Initialiser le client
	client = core.NewClient()

	// Initialiser un agent (pour tests)
	agent = core.NewAgent("test-machine")
	agent.Start()

	// Enregistrer les 3 plugins de base
	cdCmd := cd.NewCdCommand()
	dirCmd := dir.NewDirCommand()
	ipconfigCmd := &ipconfig.IpconfigCommand{}

	agent.Registry.Register(cdCmd)
	agent.Registry.Register(dirCmd)
	agent.Registry.Register(ipconfigCmd)

	// Ajouter l'agent au client
	client.Agents[agent.ID] = agent
}

var rootCmd = &cobra.Command{
	Use:   "trish",
	Short: "Trish - Remote PC Management System",
	Long:  "Trish is a backend system for managing and auditing office PCs",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
		}
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all connected agents",
	Run: func(cmd *cobra.Command, args []string) {
		agents := client.ListAgents()
		if len(agents) == 0 {
			fmt.Println("No agents connected")
			return
		}
		fmt.Println("Connected Agents:")
		for _, a := range agents {
			fmt.Printf("  - %s\n", a)
		}
	},
}

var execCmd = &cobra.Command{
	Use:   "exec <agent-id> <command> [args...]",
	Short: "Execute a command on a remote agent",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		agentID := args[0]
		cmdName := args[1]
		cmdArgs := args[2:]

		result, err := client.ExecuteOnAgent(agentID, cmdName, cmdArgs)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(result)
	},
}

var cmdCmd = &cobra.Command{
	Use:   "cmd <agent-id> <command> [args...]",
	Short: "Run a command (alias for exec)",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		agentID := args[0]
		cmdName := args[1]
		cmdArgs := args[2:]

		result, err := client.ExecuteOnAgent(agentID, cmdName, cmdArgs)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(result)
	},
}

var infoCmd = &cobra.Command{
	Use:   "info [agent-id]",
	Short: "Get information about an agent",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			// Afficher infos du premier agent
			for _, a := range client.Agents {
				fmt.Printf("Agent ID: %s\n", a.ID)
				fmt.Printf("Hostname: %s\n", a.Hostname)
				fmt.Printf("IP Address: %s\n", a.IPAddress)
				fmt.Printf("Port: %d\n", a.Port)
				fmt.Printf("Connected: %v\n", a.Connected)
				fmt.Printf("Available Commands:\n")
				for _, c := range a.Registry.List() {
					fmt.Printf("  - %s\n", c)
				}
				return
			}
		}
		agentID := args[0]
		a, exists := client.Agents[agentID]
		if !exists {
			fmt.Printf("Agent %s not found\n", agentID)
			os.Exit(1)
		}

		fmt.Printf("Agent ID: %s\n", a.ID)
		fmt.Printf("Hostname: %s\n", a.Hostname)
		fmt.Printf("IP Address: %s\n", a.IPAddress)
		fmt.Printf("Port: %d\n", a.Port)
		fmt.Printf("Connected: %v\n", a.Connected)
		fmt.Printf("Available Commands:\n")

		for _, c := range a.Registry.List() {
			fmt.Printf("  - %s\n", c)
		}
	},
}

func main() {
	rootCmd.AddCommand(listCmd, execCmd, cmdCmd, infoCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
