package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/debipro/cli/pkg/debi"
)

// addGenericCommands registers the low-level get/post/put/delete commands.
func (a *App) addGenericCommands(root *cobra.Command) {
	root.AddCommand(
		a.genericCmd("get", "GET", "Make an authenticated GET request to the Debi API"),
		a.genericCmd("post", "POST", "Make an authenticated POST request to the Debi API"),
		a.genericCmd("put", "PUT", "Make an authenticated PUT request to the Debi API"),
		a.genericCmd("delete", "DELETE", "Make an authenticated DELETE request to the Debi API"),
	)
}

func (a *App) genericCmd(use, method, short string) *cobra.Command {
	var (
		data           []string
		idempotencyKey string
		autoPaginate   bool
	)

	hasBody := method == "POST" || method == "PUT" || method == "PATCH"

	cmd := &cobra.Command{
		Use:     use + " <path>",
		Short:   short,
		Args:    cobra.ExactArgs(1),
		Example: fmt.Sprintf("  debi %s /v1/customers%s", use, exampleData(hasBody)),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.Client()
			if err != nil {
				return err
			}

			req := debi.Request{Method: method, Path: normalizePath(args[0]), IdempotencyKey: idempotencyKey}

			if hasBody {
				body, err := buildBody(data)
				if err != nil {
					return err
				}
				req.Body = body
				if req.IdempotencyKey == "" && method == "POST" {
					req.IdempotencyKey = newIdempotencyKey()
				}
			} else {
				q, err := buildQuery(data)
				if err != nil {
					return err
				}
				req.Query = q
			}

			if autoPaginate && method == "GET" {
				return a.autoPaginate(withContext(), client, req)
			}

			resp, err := client.Do(withContext(), req)
			if err != nil {
				return err
			}
			return a.PrintResponse(resp)
		},
	}

	cmd.Flags().StringArrayVarP(&data, "data", "d", nil, "request data as key=value (string) or key:=json (raw JSON); repeatable")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "value for the Idempotency-Key header (POST only; auto-generated if omitted)")
	if method == "GET" {
		cmd.Flags().BoolVar(&autoPaginate, "auto-paginate", false, "follow pagination and return all results")
	}
	return cmd
}

func exampleData(hasBody bool) string {
	if hasBody {
		return " -d name=\"Jane Doe\" -d email=jane@example.com"
	}
	return " -d limit=5"
}

func normalizePath(p string) string {
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// listEnvelope captures the pagination-relevant fields of a list response.
type listEnvelope struct {
	Data  []json.RawMessage `json:"data"`
	Links struct {
		Next string `json:"next"`
	} `json:"links"`
	Meta struct {
		NextCursor *string `json:"next_cursor"`
	} `json:"meta"`
}

// autoPaginate follows pagination links/cursors and prints a single combined
// {"data": [...]} document.
func (a *App) autoPaginate(ctx context.Context, client *debi.Client, req debi.Request) error {
	var all []json.RawMessage

	resp, err := client.Do(ctx, req)
	if err != nil {
		return err
	}

	for {
		var env listEnvelope
		if err := json.Unmarshal(resp.Body, &env); err != nil {
			// Not a list response; just print what we got.
			return a.PrintResponse(resp)
		}
		all = append(all, env.Data...)

		switch {
		case env.Links.Next != "":
			resp, err = client.DoURL(ctx, "GET", env.Links.Next)
		case env.Meta.NextCursor != nil && *env.Meta.NextCursor != "" && len(env.Data) > 0:
			if req.Query == nil {
				req.Query = map[string][]string{}
			}
			req.Query.Set("starting_after", *env.Meta.NextCursor)
			resp, err = client.Do(ctx, req)
		default:
			combined, merr := json.Marshal(map[string]interface{}{"data": all})
			if merr != nil {
				return merr
			}
			return a.printRaw(combined)
		}
		if err != nil {
			return err
		}
	}
}
