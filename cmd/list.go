package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/ptone/scion-agent/pkg/agent"
	"github.com/ptone/scion-agent/pkg/api"
	"github.com/ptone/scion-agent/pkg/config"
	"github.com/ptone/scion-agent/pkg/runtime"
	"github.com/spf13/cobra"
)

var (
	listAll bool
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List running scion agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		rt := runtime.GetRuntime(grovePath, profile)
		mgr := agent.NewManager(rt)

		filters := map[string]string{
			"scion.agent": "true",
		}

		if listAll {
			// Cross-grove listing might need a way to find all groves.
			// For now, mgr.List handles current grove and what's provided in filters.
		} else {
			projectDir, _ := config.GetResolvedProjectDir(grovePath)
			if projectDir != "" {
				filters["scion.grove_path"] = projectDir
				filters["scion.grove"] = config.GetGroveName(projectDir)
			}
		}

		agents, err := mgr.List(context.Background(), filters)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			if agents == nil {
				agents = []api.AgentInfo{}
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(agents)
		}

		if len(agents) == 0 {
			if listAll {
				fmt.Println("No active agents found across any groves.")
			} else {
				fmt.Println("No active agents found in the current grove.")
			}
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTEMPLATE\tRUNTIME\tGROVE\tAGENT STATUS\tSESSION\tCONTAINER")
		for _, a := range agents {
			agentStatus := a.Status
			if agentStatus == "" {
				agentStatus = "IDLE"
			}
			sessionStatus := a.SessionStatus
			if sessionStatus == "" {
				sessionStatus = "-"
			}
			containerStatus := a.ContainerStatus
			if containerStatus == "created" && a.ID == "" {
				containerStatus = "none"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", a.Name, a.Template, a.Runtime, a.Grove, agentStatus, sessionStatus, containerStatus)
		}
		w.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVarP(&listAll, "all", "a", false, "List all agents across all groves")
}
