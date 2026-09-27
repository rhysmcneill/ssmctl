package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/app"
	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

func paramCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "param",
		Short: "Manage SSM Parameter Store values",
		Long: `Get, list, put, and delete AWS Systems Manager Parameter Store parameters.

SecureString parameters are automatically decrypted on get and list.`,
	}
	cmd.AddCommand(paramGetCmd(), paramListCmd(), paramPutCmd(), paramDeleteCmd())
	return cmd
}

// ── get ───────────────────────────────────────────────────────────────────────

func paramGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Get a parameter's value",
		Long: `Fetch a single SSM parameter, decrypting SecureString values automatically.

Text output prints only the value — no label — making it pipe-friendly:

  export DB_PASS=$(ssmctl param get /myapp/prod/DB_PASSWORD)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := cmd.Context().Value(app.ContextKey{}).(*app.App)

			param, err := ssmlib.GetParameter(cmd.Context(), a.ParamClient, args[0])
			if err != nil {
				return fmt.Errorf("get parameter: %w", err)
			}

			if a.Config.Output == "json" {
				return a.Printer.Print(param)
			}

			if _, err = fmt.Fprintln(cmd.OutOrStdout(), param.Value); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
}

// ── list ──────────────────────────────────────────────────────────────────────

type paramListOptions struct {
	recursive bool
}

func paramListCmd() *cobra.Command {
	opts := &paramListOptions{}

	cmd := &cobra.Command{
		Use:   "list <path>",
		Short: "List parameters under a path prefix",
		Long: `List all SSM parameters whose name begins with the given path prefix.

Use --recursive to also include parameters in sub-paths:

  ssmctl param list /myapp/prod/
  ssmctl param list /myapp/ --recursive`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := cmd.Context().Value(app.ContextKey{}).(*app.App)

			params, err := ssmlib.ListParameters(cmd.Context(), a.ParamClient, args[0], opts.recursive)
			if err != nil {
				return fmt.Errorf("list parameters: %w", err)
			}

			if a.Config.Output == "json" {
				return a.Printer.Print(params)
			}

			return printParamTable(cmd.OutOrStdout(), params)
		},
	}

	cmd.Flags().BoolVar(&opts.recursive, "recursive", false, "List all parameters under the path recursively")

	return cmd
}

func printParamTable(out io.Writer, params []ssmlib.Parameter) error {
	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)

	fmt.Fprintln(w, "NAME\tTYPE\tVERSION\tLAST MODIFIED") //nolint:errcheck // tabwriter buffers; errors surface on Flush
	for _, p := range params {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", p.Name, p.Type, p.Version, p.LastModifiedDate) //nolint:errcheck // tabwriter buffers; errors surface on Flush
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush table output: %w", err)
	}
	return nil
}

// ── put ───────────────────────────────────────────────────────────────────────

type paramPutOptions struct {
	paramType string
	overwrite bool
}

func paramPutCmd() *cobra.Command {
	opts := &paramPutOptions{}

	cmd := &cobra.Command{
		Use:   "put <name> <value>",
		Short: "Create or update a parameter",
		Long: `Create a new SSM parameter or update an existing one.

The parameter type defaults to String. Use --type SecureString to encrypt
the value with the account's default KMS key.

To update an existing parameter you must pass --overwrite:

  ssmctl param put /myapp/prod/DB_PASSWORD "newvalue" --overwrite`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := cmd.Context().Value(app.ContextKey{}).(*app.App)

			switch opts.paramType {
			case "String", "StringList", "SecureString":
			default:
				return fmt.Errorf("invalid --type %q: must be String, StringList, or SecureString", opts.paramType)
			}

			result, err := ssmlib.PutParameter(cmd.Context(), a.ParamClient, args[0], args[1], opts.paramType, opts.overwrite)
			if err != nil {
				return fmt.Errorf("put parameter: %w", err)
			}

			if a.Config.Output == "json" {
				return a.Printer.Print(result)
			}

			if _, err = fmt.Fprintf(cmd.OutOrStdout(), "Parameter %s set (version %d)\n", args[0], result.Version); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&opts.paramType, "type", "String", "Parameter type: String, StringList, or SecureString")
	cmd.Flags().BoolVar(&opts.overwrite, "overwrite", false, "Overwrite the parameter if it already exists")

	return cmd
}

// ── delete ────────────────────────────────────────────────────────────────────

type paramDeleteResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func paramDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a parameter",
		Long:  `Permanently delete an SSM parameter by its full name.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := cmd.Context().Value(app.ContextKey{}).(*app.App)

			if err := ssmlib.DeleteParameter(cmd.Context(), a.ParamClient, args[0]); err != nil {
				return fmt.Errorf("delete parameter: %w", err)
			}

			if a.Config.Output == "json" {
				return a.Printer.Print(paramDeleteResult{Name: args[0], Status: "deleted"})
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Deleted parameter %s\n", args[0]); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
}
