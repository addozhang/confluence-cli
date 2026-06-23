package cli

import (
	"github.com/spf13/cobra"

	"github.com/addozhang/cfl/internal/auth"
	cflerrors "github.com/addozhang/cfl/internal/errors"
	"github.com/addozhang/cfl/internal/output"
	"github.com/addozhang/cfl/internal/schema"
)

// newSpaceCmd builds the `cfl space` command group.
func newSpaceCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "space",
		Short: "List and read Confluence spaces",
	}
	cmd.AddCommand(newSpaceListCmd(deps))
	cmd.AddCommand(newSpaceGetCmd(deps))
	return cmd
}

func newSpaceListCmd(deps *Deps) *cobra.Command {
	var limit, start int
	var instance string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Confluence spaces (single bounded page)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := deps.LoadStore()
			if err != nil {
				return err
			}
			target, err := singleInstanceOr(instance, store)
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

			raw, err := client.ListSpaces(cmd.Context(), limit, start)
			if err != nil {
				return cflerrors.TranslateConfluence(ref.HostKey(), cflerrors.ResourceSpace, err)
			}
			if deps.OutputFormat == output.FormatRaw {
				return output.Write(cmd.OutOrStdout(), output.Raw(raw), output.FormatRaw)
			}
			list, err := schema.MapSpaceList(raw)
			if err != nil {
				return cflerrors.TranslateMalformed(ref.HostKey(), err)
			}
			return output.Write(cmd.OutOrStdout(), list, deps.OutputFormat)
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "l", 0, "max spaces to return (server default when omitted)")
	cmd.Flags().IntVar(&start, "start", 0, "pagination start offset (server default when omitted)")
	cmd.Flags().StringVarP(&instance, "instance", "i", "", "instance URL or alias (optional when a single instance is configured)")
	return cmd
}

func newSpaceGetCmd(deps *Deps) *cobra.Command {
	var instance string
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Read a single Confluence space by key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			store, err := deps.LoadStore()
			if err != nil {
				return err
			}
			target, err := singleInstanceOr(instance, store)
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

			raw, err := client.GetSpace(cmd.Context(), key)
			if err != nil {
				return cflerrors.TranslateConfluence(ref.HostKey(), cflerrors.ResourceSpace, err)
			}
			if deps.OutputFormat == output.FormatRaw {
				return output.Write(cmd.OutOrStdout(), output.Raw(raw), output.FormatRaw)
			}
			detail, err := schema.MapSpace(raw)
			if err != nil {
				return cflerrors.TranslateMalformed(ref.HostKey(), err)
			}
			return output.Write(cmd.OutOrStdout(), detail, deps.OutputFormat)
		},
	}
	cmd.Flags().StringVarP(&instance, "instance", "i", "", "instance URL or alias (optional when a single instance is configured)")
	return cmd
}

// singleInstanceOr returns the explicit instance URL when provided, otherwise
// the single configured instance key, or an error when zero or multiple are
// configured (space commands carry no host in their argument).
func singleInstanceOr(instance string, store *auth.Store) (string, error) {
	if instance != "" {
		return instance, nil
	}
	keys := store.List()
	switch len(keys) {
	case 0:
		return "", &cflerrors.CFLError{
			Code:       cflerrors.CodeConfig,
			Message:    "No Confluence instance is configured.",
			Suggestion: "Run `cfl auth add <url>` to configure an instance, or pass --instance <url>.",
		}
	case 1:
		return keys[0], nil
	default:
		return "", &cflerrors.CFLError{
			Code:       cflerrors.CodeConfig,
			Message:    "Multiple instances are configured; the target is ambiguous.",
			Suggestion: "Pass --instance <url> to choose the instance.",
		}
	}
}
