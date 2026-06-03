package cmd

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/debipro/cli/pkg/debi"
	"github.com/debipro/cli/pkg/spec"
)

// reservedFlags must not be shadowed by generated resource flags.
var reservedFlags = map[string]bool{
	"data": true, "idempotency-key": true, "auto-paginate": true, "help": true,
	"json": true, "live": true, "test": true, "profile": true, "config": true,
	"api-key": true, "api-version": true, "no-color": true, "verbose": true,
}

// addResourceCommands builds the resource command tree from the embedded spec.
func (a *App) addResourceCommands(root *cobra.Command) error {
	s, err := a.loadSpec()
	if err != nil {
		return fmt.Errorf("loading OpenAPI spec: %w", err)
	}

	cache := map[string]*cobra.Command{}
	paths := make([]string, 0, len(s.Paths))
	for p := range s.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, path := range paths {
		item := s.Paths[path]
		for _, mo := range item.Operations() {
			info := analyzePath(path, mo.Method)
			if len(info.Namespace) == 0 {
				continue
			}
			parent := ensureNamespace(root, cache, info.Namespace)

			leaf := info.Leaf
			if findChild(parent, leaf) != nil {
				leaf = leaf + "-" + strings.ToLower(mo.Method)
			}
			parent.AddCommand(a.buildResourceCommand(path, mo.Method, mo.Operation, leaf, info.PathParams))
		}
	}
	return nil
}

// opInfo is the result of analyzing a path + method into a command shape.
type opInfo struct {
	Namespace  []string
	Leaf       string
	PathParams []string
}

func analyzePath(path, method string) opInfo {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/v1/"), "/")
	segs := strings.Split(trimmed, "/")

	var allStatic, staticBeforeParam, pathParams []string
	seenParam := false
	action := ""

	for i := 0; i < len(segs); i++ {
		s := segs[i]
		if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
			pathParams = append(pathParams, strings.TrimSuffix(strings.TrimPrefix(s, "{"), "}"))
			seenParam = true
			continue
		}
		if s == "actions" && i+1 < len(segs) {
			action = segs[i+1]
			i++
			continue
		}
		allStatic = append(allStatic, s)
		if !seenParam {
			staticBeforeParam = append(staticBeforeParam, s)
		}
	}

	lastSeg := segs[len(segs)-1]
	endsWithParam := strings.HasPrefix(lastSeg, "{")

	switch {
	case action != "":
		return opInfo{Namespace: staticBeforeParam, Leaf: snakeCase(action), PathParams: pathParams}
	case endsWithParam:
		return opInfo{Namespace: allStatic, Leaf: verbForItem(method), PathParams: pathParams}
	}

	last := allStatic[len(allStatic)-1]
	switch {
	case last == "search":
		return opInfo{Namespace: allStatic[:len(allStatic)-1], Leaf: "search", PathParams: pathParams}
	case seenParam:
		return opInfo{Namespace: staticBeforeParam, Leaf: snakeCase(last), PathParams: pathParams}
	default:
		return opInfo{Namespace: allStatic, Leaf: verbForCollection(method), PathParams: pathParams}
	}
}

func verbForItem(method string) string {
	switch method {
	case "GET":
		return "retrieve"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	default:
		return strings.ToLower(method)
	}
}

func verbForCollection(method string) string {
	switch method {
	case "GET":
		return "list"
	case "POST":
		return "create"
	default:
		return strings.ToLower(method)
	}
}

func (a *App) buildResourceCommand(path, method string, op *spec.Operation, leaf string, pathParams []string) *cobra.Command {
	use := leaf
	for _, p := range pathParams {
		use += " <" + p + ">"
	}

	var (
		data           []string
		idempotencyKey string
		autoPaginate   bool
		querySetters   []func(*cobra.Command, url.Values)
		bodySetters    []func(*cobra.Command, map[string]interface{})
		headerSetters  []func(*cobra.Command, http.Header)
	)

	hasBody := method == "POST" || method == "PUT" || method == "PATCH"

	cmd := &cobra.Command{
		Use:   use,
		Short: shortFor(op),
		Long:  collapse(op.Description),
		Args:  cobra.ExactArgs(len(pathParams)),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.Client()
			if err != nil {
				return err
			}

			req := debi.Request{
				Method:         method,
				Path:           substitutePath(path, pathParams, args),
				IdempotencyKey: idempotencyKey,
			}

			q := url.Values{}
			for _, set := range querySetters {
				set(cmd, q)
			}
			if !hasBody {
				extra, err := buildQuery(data)
				if err != nil {
					return err
				}
				for k, vs := range extra {
					q[k] = append(q[k], vs...)
				}
			}
			if len(q) > 0 {
				req.Query = q
			}

			headers := http.Header{}
			for _, set := range headerSetters {
				set(cmd, headers)
			}
			if len(headers) > 0 {
				req.Headers = headers
			}

			if hasBody {
				body, err := buildBody(data)
				if err != nil {
					return err
				}
				if body == nil {
					body = map[string]interface{}{}
				}
				for _, set := range bodySetters {
					set(cmd, body)
				}
				if len(body) > 0 {
					req.Body = body
				}
				if req.IdempotencyKey == "" && method == "POST" {
					req.IdempotencyKey = newIdempotencyKey()
				}
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

	flags := cmd.Flags()

	for _, p := range op.Parameters {
		name := p.Name
		if reservedFlags[name] || flags.Lookup(name) != nil {
			continue
		}
		desc := schemaDesc(p.Schema, trimDesc(p.Description))
		switch p.In {
		case "query":
			a.addQueryFlag(flags, name, desc, p.Schema, &querySetters)
		case "header":
			if name == "Idempotency-Key" {
				continue
			}
			a.addHeaderFlag(flags, name, desc, p.Schema, &headerSetters)
		}
		if p.Required {
			_ = cmd.MarkFlagRequired(name)
		}
	}

	if hasBody {
		if schema := op.RequestBody.JSONSchema(); schema != nil {
			required := map[string]bool{}
			for _, r := range schema.Required {
				required[r] = true
			}
			for _, prop := range schema.SortedProperties() {
				ps := schema.Properties[prop]
				if reservedFlags[prop] || flags.Lookup(prop) != nil {
					continue
				}
				desc := schemaDesc(ps, trimDesc(ps.Description))
				a.addBodyFlag(flags, prop, desc, ps, &bodySetters)
				if required[prop] {
					_ = cmd.MarkFlagRequired(prop)
				}
			}
		}
	}

	flags.StringArrayVarP(&data, "data", "d", nil, "extra request data as key=value or key:=json; repeatable")
	if hasBody {
		flags.StringVar(&idempotencyKey, "idempotency-key", "", "value for the Idempotency-Key header (auto-generated for POST)")
	}
	if method == "GET" {
		flags.BoolVar(&autoPaginate, "auto-paginate", false, "follow pagination and return all results")
	}

	return cmd
}

func schemaDesc(s *spec.Schema, base string) string {
	if s == nil || len(s.Enum) == 0 {
		return base
	}
	values := make([]string, 0, len(s.Enum))
	for _, v := range s.Enum {
		values = append(values, fmt.Sprint(v))
	}
	sort.Strings(values)
	return base + " (allowed: " + strings.Join(values, ", ") + ")"
}

func (a *App) addQueryFlag(flags *pflag.FlagSet, name, desc string, schema *spec.Schema, setters *[]func(*cobra.Command, url.Values)) {
	switch schemaType(schema) {
	case "boolean":
		val := new(bool)
		flags.BoolVar(val, name, false, desc)
		*setters = append(*setters, func(c *cobra.Command, q url.Values) {
			if c.Flags().Changed(name) {
				q.Set(name, strconv.FormatBool(*val))
			}
		})
	case "array":
		var vals []string
		flags.StringArrayVar(&vals, name, nil, desc)
		*setters = append(*setters, func(c *cobra.Command, q url.Values) {
			if c.Flags().Changed(name) {
				q[name] = append([]string(nil), vals...)
			}
		})
	default:
		val := new(string)
		flags.StringVar(val, name, "", desc)
		*setters = append(*setters, func(c *cobra.Command, q url.Values) {
			if c.Flags().Changed(name) {
				q.Set(name, *val)
			}
		})
	}
}

func (a *App) addHeaderFlag(flags *pflag.FlagSet, name, desc string, schema *spec.Schema, setters *[]func(*cobra.Command, http.Header)) {
	val := new(string)
	flags.StringVar(val, name, "", desc)
	*setters = append(*setters, func(c *cobra.Command, h http.Header) {
		if c.Flags().Changed(name) {
			h.Set(name, *val)
		}
	})
	_ = schemaType(schema)
}

func (a *App) addBodyFlag(flags *pflag.FlagSet, prop, desc string, ps *spec.Schema, setters *[]func(*cobra.Command, map[string]interface{})) {
	switch schemaType(ps) {
	case "integer":
		val := new(int64)
		flags.Int64Var(val, prop, 0, desc)
		*setters = append(*setters, scalarSetter(prop, func() (interface{}, bool) { return *val, true }))
	case "number":
		val := new(float64)
		flags.Float64Var(val, prop, 0, desc)
		*setters = append(*setters, scalarSetter(prop, func() (interface{}, bool) { return *val, true }))
	case "boolean":
		val := new(bool)
		flags.BoolVar(val, prop, false, desc)
		*setters = append(*setters, scalarSetter(prop, func() (interface{}, bool) { return *val, true }))
	case "array":
		var vals []string
		flags.StringArrayVar(&vals, prop, nil, desc)
		*setters = append(*setters, func(c *cobra.Command, body map[string]interface{}) {
			if c.Flags().Changed(prop) {
				items := make([]interface{}, len(vals))
				for i, s := range vals {
					items[i] = s
				}
				setNested(body, prop, items)
			}
		})
	default:
		val := new(string)
		flags.StringVar(val, prop, "", desc)
		*setters = append(*setters, scalarSetter(prop, func() (interface{}, bool) { return *val, true }))
	}
}

func schemaType(s *spec.Schema) string {
	if s == nil {
		return "string"
	}
	if s.Type.Primary() == "array" && s.Items != nil {
		return "array"
	}
	return s.Type.Primary()
}

// scalarSetter writes the named property into the body when its flag changed.
func scalarSetter(prop string, get func() (interface{}, bool)) func(*cobra.Command, map[string]interface{}) {
	return func(c *cobra.Command, body map[string]interface{}) {
		if c.Flags().Changed(prop) {
			if v, ok := get(); ok {
				setNested(body, prop, v)
			}
		}
	}
}

func substitutePath(path string, params, args []string) string {
	out := path
	for i, name := range params {
		if i < len(args) {
			out = strings.Replace(out, "{"+name+"}", url.PathEscape(args[i]), 1)
		}
	}
	return out
}

func ensureNamespace(root *cobra.Command, cache map[string]*cobra.Command, parts []string) *cobra.Command {
	parent := root
	key := ""
	for _, part := range parts {
		if key == "" {
			key = part
		} else {
			key += " " + part
		}
		if c, ok := cache[key]; ok {
			parent = c
			continue
		}
		c := findChild(parent, part)
		if c == nil {
			c = &cobra.Command{Use: part, Short: "Manage " + strings.ReplaceAll(part, "_", " ")}
			parent.AddCommand(c)
		}
		cache[key] = c
		parent = c
	}
	return parent
}

func findChild(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

func shortFor(op *spec.Operation) string {
	if op.Summary != "" {
		return op.Summary
	}
	return trimDesc(op.Description)
}

func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// trimDesc produces a short single-line flag/command description.
func trimDesc(s string) string {
	s = collapse(s)
	const max = 110
	if len(s) > max {
		s = s[:max] + "..."
	}
	return s
}

// collapse flattens whitespace in a string to single spaces.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
