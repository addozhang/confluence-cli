package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	cflerrors "github.com/addozhang/cfl/internal/errors"
	"github.com/addozhang/cfl/internal/output"
	"github.com/addozhang/cfl/internal/schema"
)

// cqlQuote wraps s in double quotes, backslash-escaping embedded backslashes and
// quotes so the value is a single CQL string literal that cannot break out of
// the quotes or inject additional CQL.
func cqlQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// buildCQL compiles the friendly search inputs into a CQL query. text (when
// non-empty) becomes a quoted free-text match; space and contentType become
// equality clauses; all clauses are AND-joined. The text is always quoted and
// escaped so it cannot alter the query structure.
func buildCQL(text, space, contentType string) string {
	clauses := make([]string, 0, 3)
	if contentType != "" {
		clauses = append(clauses, "type = "+cqlQuote(contentType))
	}
	if space != "" {
		clauses = append(clauses, "space = "+cqlQuote(space))
	}
	if text != "" {
		clauses = append(clauses, "text ~ "+cqlQuote(text))
	}
	return strings.Join(clauses, " AND ")
}

// newSearchCmd builds `cfl search`.
func newSearchCmd(deps *Deps) *cobra.Command {
	var space, contentType, cql, instance string
	var limit, start int

	cmd := &cobra.Command{
		Use:   "search [text]",
		Short: "Search Confluence content (CQL under the hood)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text := ""
			if len(args) == 1 {
				text = args[0]
			}

			// --cql has the highest precedence: when set it is the complete
			// query and the friendly inputs are ignored (not rejected).
			query := cql
			if cql != "" {
				if text != "" || space != "" || contentType != "" {
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(),
						"note: --cql takes precedence; ignoring the search term, --space, and --type")
				}
			} else {
				if contentType == "" {
					contentType = "page"
				}
				query = buildCQL(text, space, contentType)
				if query == "" {
					return &cflerrors.CFLError{
						Code:       cflerrors.CodeBadURL,
						Message:    "Nothing to search for.",
						Suggestion: "Pass a search term, --space, --type, or a raw --cql query.",
					}
				}
			}

			store, err := deps.LoadStore()
			if err != nil {
				return err
			}
			target, err := singleInstanceOr(instance, store)
			if err != nil {
				return err
			}
			// Expand an alias in the resolved target before building the client.
			target, err = resolveInstance(target, store)
			if err != nil {
				return err
			}
			if err := requireCredential(target, store); err != nil {
				return err
			}
			ref, err := refForInstance(target)
			if err != nil {
				return err
			}
			client, err := deps.ClientForRef(ref, store)
			if err != nil {
				return err
			}

			raw, err := client.Search(cmd.Context(), query, limit, start)
			if err != nil {
				return cflerrors.TranslateConfluence(ref.HostKey(), cflerrors.ResourcePage, err)
			}
			if deps.OutputFormat == output.FormatRaw {
				return output.Write(cmd.OutOrStdout(), output.Raw(raw), output.FormatRaw)
			}
			results, err := schema.MapSearch(ref.BaseURL+ref.ContextPath, raw)
			if err != nil {
				return cflerrors.TranslateMalformed(ref.HostKey(), err)
			}
			return output.Write(cmd.OutOrStdout(), results, deps.OutputFormat)
		},
	}
	cmd.Flags().StringVar(&space, "space", "", "restrict to a space key")
	cmd.Flags().StringVar(&contentType, "type", "", "content type: page or blogpost (default page)")
	cmd.Flags().StringVar(&cql, "cql", "", "raw CQL query (highest precedence; overrides the term/--space/--type)")
	cmd.Flags().IntVar(&limit, "limit", 0, "max results (server default when omitted)")
	cmd.Flags().IntVar(&start, "start", 0, "pagination start offset (server default when omitted)")
	cmd.Flags().StringVar(&instance, "instance", "", "instance URL or alias (optional when a single instance is configured)")
	return cmd
}
