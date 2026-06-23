package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/addozhang/cfl/internal/confluence"
	"github.com/addozhang/cfl/internal/confluenceurl"
	cflerrors "github.com/addozhang/cfl/internal/errors"
	"github.com/addozhang/cfl/internal/output"
	"github.com/addozhang/cfl/internal/schema"
)

// newPageCmd builds the `cfl page` command group.
func newPageCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "page",
		Short: "Read and manage Confluence pages",
	}
	cmd.AddCommand(newPageGetCmd(deps))
	cmd.AddCommand(newPageCreateCmd(deps))
	cmd.AddCommand(newPageUpdateCmd(deps))
	cmd.AddCommand(newPageDeleteCmd(deps))
	cmd.AddCommand(newPageChildrenCmd(deps))
	return cmd
}

// addInstanceFlag registers the shared --instance flag on a page command. It
// selects the target instance (URL or alias) for a bare page ID; it is ignored
// when the argument is a full URL or an <alias>:<id> form.
func addInstanceFlag(cmd *cobra.Command, instance *string) {
	cmd.Flags().StringVar(instance, "instance", "",
		"instance URL or alias, required for a bare page ID when several instances are configured (ignored for full URLs)")
}

// prepare resolves an argument into a ref, a client, and the concrete page id,
// performing the display-URL title lookup when the ref carries a space+title
// instead of an id. It centralizes the auth check and the common resolution
// flow shared by every page subcommand. The instance argument selects the target
// instance for a bare page ID (ignored when the argument is a full URL or an
// <alias>:<id> form, which carry their own host).
func prepare(cmd *cobra.Command, deps *Deps, arg, instance string) (*confluence.Client, confluenceurl.Ref, string, error) {
	store, err := deps.LoadStore()
	if err != nil {
		return nil, confluenceurl.Ref{}, "", err
	}

	// Resolve the argument (+ optional --instance) into a Ref. resolveTarget
	// handles URLs, bare ids (against --instance or the single instance), and
	// the <alias>:<id> form uniformly, producing a concrete base URL/host.
	ref, err := resolveTarget(arg, instance, store)
	if err != nil {
		return nil, confluenceurl.Ref{}, "", err
	}
	// Now that we know the host, require a stored credential for it.
	if err := requireCredential(ref.HostKey(), store); err != nil {
		return nil, confluenceurl.Ref{}, "", err
	}

	client, err := deps.ClientForRef(ref, store)
	if err != nil {
		return nil, confluenceurl.Ref{}, "", err
	}

	id := ref.PageID
	if id == "" {
		// Display URL: resolve the page id by space key + title.
		if ref.SpaceKey == "" || ref.Title == "" {
			return nil, confluenceurl.Ref{}, "", &cflerrors.CFLError{
				Code:       cflerrors.CodeBadURL,
				Message:    fmt.Sprintf("Cannot determine a page from %q.", arg),
				Suggestion: "Pass a pageId URL, a numeric page ID, or a display URL with a page title.",
			}
		}
		resolvedID, lerr := lookupPageID(cmd, deps, client, ref)
		if lerr != nil {
			return nil, confluenceurl.Ref{}, "", lerr
		}
		id = resolvedID
	}
	return client, ref, id, nil
}

// lookupPageID resolves a display-URL page (space key + title) to its numeric
// id via a title lookup.
func lookupPageID(cmd *cobra.Command, _ *Deps, client *confluence.Client, ref confluenceurl.Ref) (string, error) {
	raw, err := client.LookupPageByTitle(cmd.Context(), ref.SpaceKey, ref.Title)
	if err != nil {
		return "", cflerrors.TranslateConfluence(ref.HostKey(), cflerrors.ResourcePage, err)
	}
	id, found, err := schema.FirstContentID(raw)
	if err != nil {
		return "", cflerrors.TranslateMalformed(ref.HostKey(), err)
	}
	if !found {
		return "", &cflerrors.CFLError{
			Code:       cflerrors.CodeNotFound,
			Message:    fmt.Sprintf("No page titled %q exists in space %s.", ref.Title, ref.SpaceKey),
			Suggestion: "Check the page title and space key, or pass a pageId URL.",
		}
	}
	return id, nil
}

func newPageGetCmd(deps *Deps) *cobra.Command {
	var instance string
	cmd := &cobra.Command{
		Use:   "get <url-or-id>",
		Short: "Read a Confluence page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ref, id, err := prepare(cmd, deps, args[0], instance)
			if err != nil {
				return err
			}
			raw, err := client.GetPage(cmd.Context(), id)
			if err != nil {
				return cflerrors.TranslateConfluence(ref.HostKey(), cflerrors.ResourcePageID(id), err)
			}
			if deps.OutputFormat == output.FormatRaw {
				return output.Write(cmd.OutOrStdout(), output.Raw(raw), output.FormatRaw)
			}
			page, err := schema.MapPage(raw)
			if err != nil {
				return cflerrors.TranslateMalformed(ref.HostKey(), err)
			}
			return output.Write(cmd.OutOrStdout(), page, deps.OutputFormat)
		},
	}
	addInstanceFlag(cmd, &instance)
	return cmd
}

func newPageCreateCmd(deps *Deps) *cobra.Command {
	var spaceKey, title, bodyFlag, parentID, instance string
	cmd := &cobra.Command{
		Use:   "create --space KEY --title T --body <input> [--parent ID]",
		Short: "Create a Confluence page",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			body, err := readBodyInput(cmd, bodyFlag)
			if err != nil {
				return err
			}

			target := instance
			if target == "" {
				// Without an explicit instance URL, fall back to the single
				// configured instance.
				store, lerr := deps.LoadStore()
				if lerr != nil {
					return lerr
				}
				keys := store.List()
				if len(keys) != 1 {
					return &cflerrors.CFLError{
						Code:       cflerrors.CodeConfig,
						Message:    "Cannot determine the target instance for create.",
						Suggestion: "Pass --instance <url>, or configure exactly one instance with `cfl auth add <url>`.",
					}
				}
				target = keys[0]
			}

			store, err := deps.LoadStore()
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

			raw, err := client.CreatePage(cmd.Context(), confluence.CreatePageInput{
				SpaceKey: spaceKey,
				Title:    title,
				Body:     body,
				ParentID: parentID,
			})
			if err != nil {
				return cflerrors.TranslateConfluence(ref.HostKey(), cflerrors.ResourcePage, err)
			}
			return writePageSummary(cmd, deps, ref.HostKey(), raw)
		},
	}
	cmd.Flags().StringVar(&spaceKey, "space", "", "space key (required)")
	cmd.Flags().StringVar(&title, "title", "", "page title (required)")
	cmd.Flags().StringVar(&bodyFlag, "body", "", "storage-format body: @file, - for stdin, or a literal string (required)")
	cmd.Flags().StringVar(&parentID, "parent", "", "parent page id (optional)")
	cmd.Flags().StringVar(&instance, "instance", "", "instance URL (optional when a single instance is configured)")
	_ = cmd.MarkFlagRequired("space")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("body")
	return cmd
}

func newPageUpdateCmd(deps *Deps) *cobra.Command {
	var bodyFlag, title, instance string
	cmd := &cobra.Command{
		Use:   "update <url-or-id> --body <input> [--title T]",
		Short: "Update a Confluence page (version-safe)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := readBodyInput(cmd, bodyFlag)
			if err != nil {
				return err
			}
			client, ref, id, err := prepare(cmd, deps, args[0], instance)
			if err != nil {
				return err
			}

			// Read the current version, then submit current+1 (never guess).
			curRaw, err := client.ReadVersion(cmd.Context(), id)
			if err != nil {
				return cflerrors.TranslateConfluence(ref.HostKey(), cflerrors.ResourcePageID(id), err)
			}
			cur, err := schema.MapPage(curRaw)
			if err != nil {
				return cflerrors.TranslateMalformed(ref.HostKey(), err)
			}
			newTitle := title
			if newTitle == "" {
				newTitle = cur.Title
			}

			raw, err := client.UpdatePage(cmd.Context(), confluence.UpdatePageInput{
				ID:         id,
				Title:      newTitle,
				Body:       body,
				NewVersion: cur.Version + 1,
			})
			if err != nil {
				return cflerrors.TranslateConfluence(ref.HostKey(), cflerrors.ResourcePageID(id), err)
			}
			return writePageSummary(cmd, deps, ref.HostKey(), raw)
		},
	}
	cmd.Flags().StringVar(&bodyFlag, "body", "", "storage-format body: @file, - for stdin, or a literal string (required)")
	cmd.Flags().StringVar(&title, "title", "", "new title (optional; preserved when omitted)")
	addInstanceFlag(cmd, &instance)
	_ = cmd.MarkFlagRequired("body")
	return cmd
}

func newPageDeleteCmd(deps *Deps) *cobra.Command {
	var yes bool
	var instance string
	cmd := &cobra.Command{
		Use:   "delete <url-or-id>",
		Short: "Move a Confluence page to the trash",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ref, id, err := prepare(cmd, deps, args[0], instance)
			if err != nil {
				return err
			}

			if !yes {
				if !isInteractive(cmd) {
					return &cflerrors.CFLError{
						Code:       cflerrors.CodeConfig,
						Message:    "Refusing to delete without confirmation in a non-interactive session. Pass --yes to confirm deletion.",
						Suggestion: "Re-run with --yes to delete without prompting.",
					}
				}
				in := bufio.NewReader(cmd.InOrStdin())
				ok, cerr := confirm(in, fmt.Sprintf("Move page %s to trash? [y/N] ", id))
				if cerr != nil || !ok {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Deletion cancelled.")
					return errDeletionCancelled
				}
			}

			if err := client.DeletePage(cmd.Context(), id); err != nil {
				return cflerrors.TranslateConfluence(ref.HostKey(), cflerrors.ResourcePageID(id), err)
			}
			return output.Write(cmd.OutOrStdout(), schema.DeleteResult{ID: id, Status: "trashed"}, deps.OutputFormat)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion without an interactive prompt")
	addInstanceFlag(cmd, &instance)
	return cmd
}

func newPageChildrenCmd(deps *Deps) *cobra.Command {
	var instance string
	cmd := &cobra.Command{
		Use:   "children <url-or-id>",
		Short: "List a page's direct children",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ref, id, err := prepare(cmd, deps, args[0], instance)
			if err != nil {
				return err
			}
			raw, err := client.GetChildren(cmd.Context(), id)
			if err != nil {
				return cflerrors.TranslateConfluence(ref.HostKey(), cflerrors.ResourcePageID(id), err)
			}
			if deps.OutputFormat == output.FormatRaw {
				return output.Write(cmd.OutOrStdout(), output.Raw(raw), output.FormatRaw)
			}
			children, err := schema.MapChildren(raw)
			if err != nil {
				return cflerrors.TranslateMalformed(ref.HostKey(), err)
			}
			return output.Write(cmd.OutOrStdout(), children, deps.OutputFormat)
		},
	}
	addInstanceFlag(cmd, &instance)
	return cmd
}

// errDeletionCancelled is returned when the user declines a delete prompt, so
// the process exits non-zero without an additional translated error line.
var errDeletionCancelled = &cflerrors.CFLError{
	Code:       cflerrors.CodeConfig,
	Message:    "Deletion cancelled.",
	Suggestion: "Re-run with --yes to delete without prompting.",
}

// writePageSummary maps a create/update response into a PageSummary and renders
// it (or passes raw bytes through under -o raw).
func writePageSummary(cmd *cobra.Command, deps *Deps, host string, raw []byte) error {
	if deps.OutputFormat == output.FormatRaw {
		return output.Write(cmd.OutOrStdout(), output.Raw(raw), output.FormatRaw)
	}
	summary, err := schema.MapPageSummary(raw)
	if err != nil {
		return cflerrors.TranslateMalformed(host, err)
	}
	return output.Write(cmd.OutOrStdout(), summary, deps.OutputFormat)
}

// readBodyInput resolves the --body flag value: "@path" reads a file, "-" reads
// stdin, anything else is the literal storage-format body. An empty flag is an
// error (the required-flag check should have caught it, but be defensive).
func readBodyInput(cmd *cobra.Command, flag string) (string, error) {
	switch {
	case flag == "":
		return "", &cflerrors.CFLError{
			Code:       cflerrors.CodeConfig,
			Message:    "Missing required --body flag.",
			Suggestion: "Pass --body @file, --body - (stdin), or a literal storage-format string.",
		}
	case flag == "-":
		b, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", fmt.Errorf("read body from stdin: %w", err)
		}
		return string(b), nil
	case strings.HasPrefix(flag, "@"):
		path := flag[1:]
		b, err := os.ReadFile(path)
		if err != nil {
			return "", &cflerrors.CFLError{
				Code:       cflerrors.CodeConfig,
				Message:    fmt.Sprintf("Could not read body file %s.", path),
				Suggestion: "Check the file path passed to --body @<path>.",
				Cause:      err,
			}
		}
		return string(b), nil
	default:
		return flag, nil
	}
}

// isInteractive reports whether stdin is a terminal, used to decide whether a
// delete may prompt for confirmation.
func isInteractive(cmd *cobra.Command) bool {
	f, ok := cmd.InOrStdin().(*os.File)
	if !ok {
		return false
	}
	return termIsTerminal(int(f.Fd()))
}
