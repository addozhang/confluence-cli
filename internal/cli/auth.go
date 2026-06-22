package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/addozhang/cfl/internal/auth"
	cflerrors "github.com/addozhang/cfl/internal/errors"
	"github.com/addozhang/cfl/internal/output"
	"github.com/addozhang/cfl/internal/schema"
)

// promptHiddenToken reads a Personal Access Token from the terminal without
// echoing it. It is a package var so tests can substitute a deterministic
// reader. The returned token is trimmed of surrounding whitespace.
var promptHiddenToken = func(prompt string) (string, error) {
	_, _ = fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	_, _ = fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read token: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

// confirm reads a yes/no answer from in, returning true only for an explicit
// affirmative. It is used for overwrite and delete confirmations.
func confirm(in *bufio.Reader, prompt string) (bool, error) {
	_, _ = fmt.Fprint(os.Stderr, prompt)
	line, err := in.ReadString('\n')
	if err != nil {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

// newAuthCmd builds the `cfl auth` command group.
func newAuthCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Confluence instance credentials",
	}
	cmd.AddCommand(newAuthAddCmd(deps))
	cmd.AddCommand(newAuthListCmd(deps))
	cmd.AddCommand(newAuthRemoveCmd(deps))
	cmd.AddCommand(newAuthWhoamiCmd(deps))
	return cmd
}

// newAuthAddCmd builds `cfl auth add <url>`.
func newAuthAddCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "add <url>",
		Short: "Store a Personal Access Token for a Confluence instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := auth.KeyFromURL(args[0])
			if err != nil {
				return cflerrors.WrapURLParse(args[0], err)
			}

			store, err := deps.LoadStore()
			if err != nil {
				return err
			}

			// Overwrite confirmation when a token already exists for this key.
			if existing := store.List(); contains(existing, key) {
				in := bufio.NewReader(cmd.InOrStdin())
				ok, cerr := confirm(in, fmt.Sprintf("A token for %s already exists. Overwrite? [y/N] ", key))
				if cerr != nil || !ok {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Aborted; existing token left unchanged.")
					return nil
				}
			}

			token, err := promptHiddenToken(fmt.Sprintf("Personal Access Token for %s: ", key))
			if err != nil {
				return err
			}
			if token == "" {
				return &cflerrors.CFLError{
					Code:       cflerrors.CodeConfig,
					Message:    "No token entered.",
					Suggestion: "Run `cfl auth add <url>` again and paste a Personal Access Token.",
				}
			}

			store.Add(key, token)
			if err := deps.SaveStore(store); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Stored token for %s\n", key)
			return nil
		},
	}
}

// newAuthListCmd builds `cfl auth list`.
func newAuthListCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured instance keys",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := deps.LoadStore()
			if err != nil {
				return err
			}
			return output.Write(cmd.OutOrStdout(), schema.NewAuthList(store.List()), deps.OutputFormat)
		},
	}
}

// newAuthRemoveCmd builds `cfl auth remove <host>`.
func newAuthRemoveCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <host>",
		Short: "Remove a stored instance credential",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := auth.KeyFromURL(args[0])
			if err != nil {
				return cflerrors.WrapURLParse(args[0], err)
			}
			store, err := deps.LoadStore()
			if err != nil {
				return err
			}
			removed := store.Remove(key)
			if err := deps.SaveStore(store); err != nil {
				return err
			}
			if removed {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Removed token for %s\n", key)
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No credential found for %s; nothing to remove.\n", key)
			}
			return nil
		},
	}
}

// newAuthWhoamiCmd builds `cfl auth whoami <url>`.
func newAuthWhoamiCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami <url>",
		Short: "Verify the stored token for an instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := deps.LoadStore()
			if err != nil {
				return err
			}
			if err := requireCredential(args[0], store); err != nil {
				return err
			}

			ref, err := refForInstance(args[0])
			if err != nil {
				return err
			}
			client, err := deps.ClientForRef(ref, store)
			if err != nil {
				return err
			}

			raw, err := client.WhoAmI(cmd.Context())
			if err != nil {
				return cflerrors.TranslateConfluence(ref.HostKey(), cflerrors.ResourcePage, err)
			}
			who, err := schema.MapWhoAmI(ref.HostKey(), raw)
			if err != nil {
				return cflerrors.TranslateMalformed(ref.HostKey(), err)
			}
			return output.Write(cmd.OutOrStdout(), who, deps.OutputFormat)
		},
	}
}

func contains(items []string, target string) bool {
	for _, s := range items {
		if s == target {
			return true
		}
	}
	return false
}
