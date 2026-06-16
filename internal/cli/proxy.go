package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/config"
)

// newProxyCmd creates the "mozza proxy" command group.
func newProxyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "Manage the Mozza reverse proxy",
		Long:  "View and manage the built-in reverse proxy routing table and certificates.",
	}

	cmd.AddCommand(newProxyStatusCmd())
	return cmd
}

// proxyRoute matches the JSON response from /api/v1/proxy/routes.
type proxyRoute struct {
	Domain         string    `json:"domain"`
	BackendURL     string    `json:"backend_url"`
	HealthEndpoint string    `json:"health_endpoint"`
	Healthy        bool      `json:"healthy"`
	LastCheck      time.Time `json:"last_check"`
}

// newProxyStatusCmd creates the "mozza proxy status" subcommand.
func newProxyStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the proxy routing table",
		Long:  "Display all registered proxy routes with their backend URLs and health status.",
		RunE:  runProxyStatus,
	}

	cmd.Flags().Int("port", config.DefaultServerPort, "API server port to query")
	cmd.Flags().String("host", "localhost", "API server host to query")

	return cmd
}

// runProxyStatus fetches and displays the current proxy routing table.
func runProxyStatus(cmd *cobra.Command, _ []string) error {
	host, _ := cmd.Flags().GetString("host")
	port, _ := cmd.Flags().GetInt("port")

	url := fmt.Sprintf("http://%s:%d/api/v1/proxy/routes", host, port)

	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("proxy status: cannot reach API server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("proxy status: API returned %d: %s", resp.StatusCode, string(body))
	}

	var routes []proxyRoute
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		return fmt.Errorf("proxy status: decode response: %w", err)
	}

	if len(routes) == 0 {
		cmd.Println("No proxy routes configured.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "DOMAIN\tBACKEND\tHEALTHY\tLAST CHECK")

	for _, r := range routes {
		healthy := "yes"
		if !r.Healthy {
			healthy = "no"
		}
		lastCheck := "never"
		if !r.LastCheck.IsZero() {
			lastCheck = r.LastCheck.Format(time.RFC3339)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Domain, r.BackendURL, healthy, lastCheck)
	}

	return w.Flush()
}
