package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/debipro/cli/pkg/debi"
	"github.com/debipro/cli/pkg/webhook"
)

func (a *App) eventsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Inspect and replay Debi events",
	}
	cmd.AddCommand(
		a.eventsTailCmd(),
		a.eventsResendCmd(),
		a.eventsVerifyCmd(),
		a.eventsTriggerCmd(),
	)
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
		interval      time.Duration
		limit         int
		eventType     string
		since         int64
		relatedObject string
		forwardTo     string
		webhookSecret string
	)

	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Poll the Events API and print new events as they arrive",
		Long: "Polls GET /v1/events on an interval and prints new events as they appear.\n" +
			"Debi does not push events to the CLI, so this is implemented via polling.\n\n" +
			"Use --forward-to to POST each new event to a local endpoint (for example\n" +
			"while `debi listen` is running). With --webhook-secret, forwarded requests\n" +
			"include a valid Debi-Signature header.",
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
			if eventType != "" {
				query["type"] = []string{eventType}
			}
			if relatedObject != "" {
				query["related_object"] = []string{relatedObject}
			}
			if since > 0 {
				query["created_at[gte]"] = []string{fmt.Sprintf("%d", since)}
			}

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
						if forwardTo != "" {
							if err := a.forwardEvent(ctx, cmd.ErrOrStderr(), client, e.ID, forwardTo, webhookSecret); err != nil {
								fmt.Fprintf(cmd.ErrOrStderr(), "forward %s: %v\n", e.ID, err)
							}
						}
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
	cmd.Flags().StringVar(&eventType, "type", "", "filter by event type (supports * wildcards)")
	cmd.Flags().Int64Var(&since, "since", 0, "only events with created_at >= this Unix timestamp")
	cmd.Flags().StringVar(&relatedObject, "related-object", "", "filter events for a specific resource ID")
	cmd.Flags().StringVar(&forwardTo, "forward-to", "", "POST each new event payload to this URL")
	cmd.Flags().StringVar(&webhookSecret, "webhook-secret", "", "sign forwarded requests with this webhook endpoint secret")
	return cmd
}

func (a *App) eventsResendCmd() *cobra.Command {
	var (
		forwardTo     string
		webhookSecret string
	)

	cmd := &cobra.Command{
		Use:   "resend <event_id>",
		Short: "Re-deliver a stored event's payload to a local endpoint",
		Long: "Fetches a stored event via GET /v1/events/{id} and re-POSTs its payload to\n" +
			"the URL given by --forward-to (client-side replay for local development).\n" +
			"Without --forward-to, the event is printed.\n\n" +
			"Security: --forward-to only accepts http://127.0.0.1, http://localhost, and\n" +
			"http://[::1] URLs by default to reduce accidental SSRF.",
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
			status, body, err := forwardJSON(withContext(), forwardTo, payload, webhookSecret)
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

	cmd.Flags().StringVar(&forwardTo, "forward-to", "", "URL to POST the event payload to (localhost only)")
	cmd.Flags().StringVar(&webhookSecret, "webhook-secret", "", "sign the forwarded request with this webhook endpoint secret")
	return cmd
}

func (a *App) eventsVerifyCmd() *cobra.Command {
	var (
		webhookSecret string
		signature     string
	)

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify a webhook payload against a Debi-Signature header",
		Long:  "Reads a JSON payload from stdin and verifies the signature header value passed via --signature.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := io.ReadAll(os.Stdin)
			if err != nil {
				return err
			}
			if webhookSecret == "" {
				return fmt.Errorf("--webhook-secret is required")
			}
			if signature == "" {
				return fmt.Errorf("--signature is required")
			}
			if err := webhook.Verify(signature, webhookSecret, payload, webhook.DefaultTolerance); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Signature valid.")
			return nil
		},
	}

	cmd.Flags().StringVar(&webhookSecret, "webhook-secret", "", "webhook endpoint signing secret")
	cmd.Flags().StringVar(&signature, "signature", "", "Debi-Signature header value to verify")
	return cmd
}

func (a *App) eventsTriggerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "trigger <scenario>",
		Short: "Run a documented test scenario that produces API events",
		Long: "Executes a built-in sandbox scenario that creates real API objects and\n" +
			"typically emits webhook events you can observe with `debi events tail`.\n\n" +
			"Available scenarios:\n" +
			"  customer.created   Create a test customer\n" +
			"  payment_method.updated  Create a test payment method (triggers automatically_updated in sandbox)",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.Client()
			if err != nil {
				return err
			}
			ctx := withContext()
			switch args[0] {
			case "customer.created":
				resp, err := client.Do(ctx, debi.Request{
					Method: "POST",
					Path:   "/v1/customers",
					Body: map[string]interface{}{
						"name":  "debi CLI test customer",
						"email": fmt.Sprintf("debi-cli+%d@example.com", time.Now().Unix()),
					},
				})
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "Created test customer. Watch with: debi events tail --type customer.created")
				return a.PrintResponse(resp)
			case "payment_method.updated":
				resp, err := client.Do(ctx, debi.Request{
					Method: "POST",
					Path:   "/v1/payment_methods",
					Body: map[string]interface{}{
						"type": "card",
						"card": map[string]interface{}{
							"number":    "4000000320000021",
							"exp_month": 12,
							"exp_year":  time.Now().Year() + 2,
							"cvc":       "123",
						},
					},
				})
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "Created test payment method. Watch with: debi events tail --type payment_method.automatically_updated")
				return a.PrintResponse(resp)
			default:
				return fmt.Errorf("unknown scenario %q (try: customer.created, payment_method.updated)", args[0])
			}
		},
	}
}

func (a *App) forwardEvent(ctx context.Context, diag io.Writer, client *debi.Client, eventID, forwardTo, webhookSecret string) error {
	resp, err := client.Do(ctx, debi.Request{
		Method: "GET",
		Path:   "/v1/events/" + eventID,
	})
	if err != nil {
		return err
	}
	payload := unwrapData(resp.Body)
	status, _, err := forwardJSON(ctx, forwardTo, payload, webhookSecret)
	if err != nil {
		return err
	}
	fmt.Fprintf(diag, "  forwarded to %s (HTTP %d)\n", forwardTo, status)
	return nil
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

// forwardJSON POSTs a JSON payload to a localhost URL.
func forwardJSON(ctx context.Context, target string, payload []byte, webhookSecret string) (int, []byte, error) {
	if err := validateForwardURL(target); err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "debi-cli")
	if webhookSecret != "" {
		req.Header.Set(webhook.HeaderName(), webhook.Sign(webhookSecret, payload, time.Now()))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body, nil
}

func validateForwardURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid forward URL: %w", err)
	}
	if u.Scheme != "http" {
		return fmt.Errorf("forward URL must use http (got %q)", u.Scheme)
	}
	host := u.Hostname()
	switch host {
	case "127.0.0.1", "localhost", "::1":
		return nil
	default:
		return fmt.Errorf("forward URL host %q is not allowed (use localhost, 127.0.0.1, or ::1)", host)
	}
}
