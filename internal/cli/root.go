package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/addozhang/cfl/internal/auth"
	"github.com/addozhang/cfl/internal/confluence"
	"github.com/addozhang/cfl/internal/confluenceurl"
	"github.com/addozhang/cfl/internal/output"
)

// Deps is the dependency bundle injected into command constructors to keep them
// testable. It is assembled from the global flags in the root command's
// PersistentPreRunE. CredentialsPath and Stderr are overridable in tests.
type Deps struct {
	OutputFormat output.Format
	Timeout      time.Duration
	Insecure     bool
	Debug        bool
	// SSLCertFile is the CA bundle path; assembled from the SSL_CERT_FILE
	// environment variable (honored with no flag, per the tls spec).
	SSLCertFile string

	// CredentialsPath is the path to the credentials file. Empty means use the
	// platform default (~/.config/cfl/credentials).
	CredentialsPath string
	// Stderr receives warnings and debug output. Defaults to os.Stderr; commands
	// set it to cmd.ErrOrStderr() so it is captured in tests and respects
	// cobra's output wiring.
	Stderr io.Writer
}

// LoadStore loads the credentials store from CredentialsPath (or the default).
func (d *Deps) LoadStore() (*auth.Store, error) {
	path, err := d.credentialsPath()
	if err != nil {
		return nil, err
	}
	return auth.Load(path)
}

// SaveStore persists the store to CredentialsPath (or the default).
func (d *Deps) SaveStore(store *auth.Store) error {
	path, err := d.credentialsPath()
	if err != nil {
		return err
	}
	return store.Save(path)
}

// credentialsPath resolves the credentials file path: the explicit
// CredentialsPath, else the CFL_TEST_CREDENTIALS test override, else the
// platform default.
func (d *Deps) credentialsPath() (string, error) {
	if d.CredentialsPath != "" {
		return d.CredentialsPath, nil
	}
	if p := os.Getenv("CFL_TEST_CREDENTIALS"); p != "" {
		return p, nil
	}
	return auth.DefaultPath()
}

// ClientForRef builds a Confluence client targeting the instance identified by
// ref, using the given store for Bearer auth and the global TLS/debug settings.
// The per-request timeout is applied through the command context (see the root
// command's PersistentPreRunE), not on the http.Client, so cancellation and
// deadlines propagate the idiomatic way.
func (d *Deps) ClientForRef(ref confluenceurl.Ref, store *auth.Store) (*confluence.Client, error) {
	stderr := d.errWriter()
	rt, err := confluence.NewTransport(confluence.TransportConfig{
		SSLCertFile: d.SSLCertFile,
		Insecure:    d.Insecure,
		Debug:       d.Debug,
		Timeout:     d.Timeout,
		Warn:        stderr,
		DebugLog:    stderr,
	}, store)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{Transport: rt}
	return confluence.NewClient(httpClient, ref.BaseURL, ref.ContextPath), nil
}

func (d *Deps) errWriter() io.Writer {
	if d.Stderr != nil {
		return d.Stderr
	}
	return os.Stderr
}

// globalFlags holds the raw flag values before they are parsed into Deps.
type globalFlags struct {
	output   string
	timeout  time.Duration
	insecure bool
	debug    bool
}

// NewRootCmd builds the root `cfl` command, registers global persistent flags,
// and wires the subcommands. main() calls Execute, which runs this.
func NewRootCmd() *cobra.Command {
	flags := &globalFlags{}
	deps := &Deps{}
	var cancelTimeout context.CancelFunc

	root := &cobra.Command{
		Use:           "cfl",
		Short:         "URL-native Confluence Server/DC CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			format, err := output.ParseFormat(flags.output)
			if err != nil {
				return err
			}
			deps.OutputFormat = format
			deps.Timeout = flags.timeout
			deps.Insecure = flags.insecure
			deps.Debug = flags.debug
			// SSL_CERT_FILE is honored with no flag (tls spec). An unset
			// CredentialsPath/Stderr keeps the defaults (and test overrides).
			deps.SSLCertFile = os.Getenv("SSL_CERT_FILE")
			if deps.Stderr == nil {
				deps.Stderr = cmd.ErrOrStderr()
			}
			// Bind the per-request timeout to the command context as a deadline,
			// so every IO call started from cmd.Context() is bounded and
			// cancellation propagates idiomatically. The cancel runs in
			// PersistentPostRun.
			if flags.timeout > 0 {
				ctx, cancel := context.WithTimeout(cmd.Context(), flags.timeout)
				cancelTimeout = cancel
				cmd.SetContext(ctx)
			}
			return nil
		},
		PersistentPostRun: func(_ *cobra.Command, _ []string) {
			if cancelTimeout != nil {
				cancelTimeout()
			}
		},
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&flags.output, "output", "o", "yaml", "output format: yaml, json, or raw")
	pf.DurationVar(&flags.timeout, "timeout", 30*time.Second, "per-request timeout (e.g. 30s, 2m)")
	pf.BoolVar(&flags.insecure, "insecure", false, "disable TLS certificate verification")
	pf.BoolVar(&flags.debug, "debug", false, "log the raw HTTP exchange to stderr (token redacted)")

	root.AddCommand(newVersionCmd(deps))
	root.AddCommand(newAuthCmd(deps))
	root.AddCommand(newPageCmd(deps))
	root.AddCommand(newSpaceCmd(deps))

	return root
}

// Execute runs the root command and returns its error for the main package to
// translate and map to an exit code. It does not print anything itself.
func Execute() error {
	if err := NewRootCmd().Execute(); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}
