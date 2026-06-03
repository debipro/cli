package cmd

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/debipro/cli/pkg/webhook"
)

func (a *App) listenCmd() *cobra.Command {
	var (
		webhookSecret string
		path          string
	)

	cmd := &cobra.Command{
		Use:   "listen [port]",
		Short: "Run a local HTTP server to receive webhook POST requests",
		Long: "Starts a local HTTP server that accepts webhook POST requests and prints\n" +
			"the payload. When --webhook-secret is set, incoming requests are verified\n" +
			"using the Debi-Signature header before being accepted.\n\n" +
			"Debi does not push events to the CLI directly; pair this with\n" +
			"`debi events tail --forward-to` or register the public URL (via ngrok,\n" +
			"cloudflared, etc.) in the Debi dashboard for end-to-end testing.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			port := "4242"
			if len(args) == 1 {
				port = args[0]
			}
			addr := net.JoinHostPort("127.0.0.1", port)

			mux := http.NewServeMux()
			mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
					return
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				if webhookSecret != "" {
					if err := webhook.Verify(r.Header.Get(webhook.HeaderName()), webhookSecret, body, webhook.DefaultTolerance); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "signature verification failed: %v\n", err)
						http.Error(w, "invalid signature", http.StatusBadRequest)
						return
					}
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Received webhook (%d bytes)\n", len(body))
				if err := a.printRaw(body); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "output error: %v\n", err)
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"received":true}`))
			})

			srv := &http.Server{Addr: addr, Handler: mux}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			go func() {
				<-ctx.Done()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = srv.Shutdown(shutdownCtx)
			}()

			fmt.Fprintf(cmd.ErrOrStderr(), "Ready! Listening for webhooks on http://%s%s\n", addr, path)
			fmt.Fprintln(cmd.ErrOrStderr(), "Press Ctrl-C to stop.")
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				return err
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "\nStopped.")
			return nil
		},
	}

	cmd.Flags().StringVar(&webhookSecret, "webhook-secret", "", "verify incoming Debi-Signature headers with this endpoint secret")
	cmd.Flags().StringVar(&path, "path", "/", "URL path to accept webhook POST requests on")
	return cmd
}
