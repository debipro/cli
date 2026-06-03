package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/tucuota/debi-cli/pkg/debi"
)

func (a *App) eventsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Inspect and replay Debi events",
	}
	cmd.AddCommand(a.eventsTailCmd(), a.eventsResendCmd())
	return cmd
}

type eventSummary struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	CreatedAt  string `json:"created_at"`
	Resource   string `json:"resource"`
	ResourceID string `json:"resource_id"`
}

func (a *App) eventsTailCmd() *cobra.Command {
	var (
		interval time.Duration
		limit    int
	)

	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Poll the Events API and print new events as they arrive",
		Long: "Polls GET /v1/events on an interval and prints new events as they appear.\n" +
			"Debi does not push events to the CLI, so this is implemented via polling.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.Client()
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			bold := color.New(color.Bold).SprintFunc()
			dim := color.New(color.Faint).SprintFunc()
			typeColor := color.New(color.FgCyan).SprintFunc()

			fmt.Fprintf(cmd.ErrOrStderr(), "Tailing events (%s environment). Press Ctrl-C to stop.\n",
				a.Mode(client.APIKey))

			seen := map[string]bool{}
			first := true

			query := map[string][]string{"limit": {fmt.Sprintf("%d", limit)}}

			poll := func() error {
				resp, err := client.Do(ctx, debi.Request{Method: "GET", Path: "/v1/events", Query: query})
				if err != nil {
					return err
				}
				var env struct {
					Data []eventSummary `json:"data"`
				}
				if err := json.Unmarshal(resp.Body, &env); err != nil {
					return err
				}
				// API returns newest first; collect unseen and print oldest first.
				var fresh []eventSummary
				for _, e := range env.Data {
					if !seen[e.ID] {
						seen[e.ID] = true
						fresh = append(fresh, e)
					}
				}
				if !first {
					for i := len(fresh) - 1; i >= 0; i-- {
						e := fresh[i]
						fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %s\n",
							dim(e.CreatedAt), typeColor(bold(e.Type)), e.ID)
					}
				}
				first = false
				return nil
			}

			if err := poll(); err != nil {
				return err
			}
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					fmt.Fprintln(cmd.ErrOrStderr(), "\nStopped.")
					return nil
				case <-ticker.C:
					if err := poll(); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "poll error: %v\n", err)
					}
				}
			}
		},
	}

	cmd.Flags().DurationVar(&interval, "interval", 3*time.Second, "polling interval")
	cmd.Flags().IntVar(&limit, "limit", 25, "number of events to fetch per poll")
	return cmd
}

func (a *App) eventsResendCmd() *cobra.Command {
	var forwardTo string

	cmd := &cobra.Command{
		Use:   "resend <event_id>",
		Short: "Re-deliver a stored event's payload to a local endpoint",
		Long: "Fetches a stored event via GET /v1/events/{id} and re-POSTs its payload to\n" +
			"the URL given by --forward-to (client-side replay for local development).\n" +
			"Without --forward-to, the event is printed.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.Client()
			if err != nil {
				return err
			}

			resp, err := client.Do(withContext(), debi.Request{
				Method: "GET",
				Path:   "/v1/events/" + args[0],
			})
			if err != nil {
				return err
			}

			if forwardTo == "" {
				return a.PrintResponse(resp)
			}

			payload := unwrapData(resp.Body)
			status, body, err := forwardJSON(withContext(), forwardTo, payload)
			if err != nil {
				return fmt.Errorf("forwarding event: %w", err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Forwarded event %s to %s (HTTP %d)\n", args[0], forwardTo, status)
			if len(bytes.TrimSpace(body)) > 0 {
				return a.printRaw(body)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&forwardTo, "forward-to", "", "URL to POST the event payload to")
	return cmd
}

// unwrapData returns the inner object of a {"data": {...}} envelope, or the
// original body if it is not wrapped.
func unwrapData(body []byte) []byte {
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err == nil && len(env.Data) > 0 {
		return env.Data
	}
	return body
}

// forwardJSON POSTs a JSON payload to an arbitrary URL.
func forwardJSON(ctx context.Context, target string, payload []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", target, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "debi-cli")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body, nil
}
