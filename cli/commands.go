package cli

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"gnorm.org/gnorm/environ"
	"gnorm.org/gnorm/run"
)

var (
	version    = "DEV"
	timestamp  = "no timestamp, did you build with make.go?"
	commitHash = "no hash, did you build with make.go?"
)

func previewCmd(env environ.Values) *cobra.Command {
	var cfgFile string
	var verbose bool
	var format string
	preview := &cobra.Command{
		Use:   "preview",
		Short: "Preview the data that will be sent to your templates",
		Long: `
Reads your gnorm.toml file and connects to your database, translating the schema
just as it would be during a full run.  It is then printed out in an
easy-to-read format.`[1:],
		RunE: func(cmd *cobra.Command, args []string) error {
			env.InitLog(verbose)
			cfg, err := parseFile(env, cfgFile)
			if err != nil {
				return codeErr{err, 2}
			}
			if err := run.Preview(env, cfg, format); err != nil {
				return codeErr{err, 1}
			}
			return nil
		},
		Args: cobra.ExactArgs(0),
	}
	preview.Flags().StringVarP(&cfgFile, "config", "c", "gnorm.toml", "relative path to gnorm config file")
	preview.Flags().StringVarP(&format, "format", "f", "tabular", "Specify output format e.g yaml, types or tabular")
	preview.Flags().BoolVarP(&verbose, "verbose", "v", false, "show debugging output")
	return preview
}

func genCmd(env environ.Values) *cobra.Command {
	var cfgFile string
	var verbose bool
	gen := &cobra.Command{
		Use:   "gen",
		Short: "Generate code from DB schema",
		Long: `
Reads your gnorm.toml file and connects to your database, translating the schema
into in-memory objects.  Then reads your templates and writes files to disk
based on those templates.`[1:],
		RunE: func(cmd *cobra.Command, args []string) error {
			env.InitLog(verbose)
			cfg, err := parseFile(env, cfgFile)
			if err != nil {
				return codeErr{err, 2}
			}
			if err := run.Generate(env, cfg); err != nil {
				return codeErr{err, 1}
			}
			return nil
		},
		Args: cobra.ExactArgs(0),
	}
	gen.Flags().StringVarP(&cfgFile, "config", "c", "gnorm.toml", "relative path to gnorm config file")
	gen.Flags().BoolVarP(&verbose, "verbose", "v", false, "show debugging output")
	return gen
}

func versionCmd(env environ.Values) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Displays the version of GNORM.",
		Long: `
Shows the build date and commit hash used to build this binary.`[1:],
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(env.Stdout, "version: %s\nbuilt at: %s\ncommit hash: %s", version, timestamp, commitHash)
		},
	}
}

func initCmd(env environ.Values) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generates the files needed to run GNORM.",
		Long: `
Creates a default gnorm.toml and the various template files needed to run GNORM.`[1:],
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := createFile("gnorm.toml", sample); err != nil {
				fmt.Fprintf(env.Stdout, "Can't create gnorm.toml file: %v\n", err)
				return codeErr{err, 1}
			}
			for _, name := range []string{"table", "schema", "enum"} {
				if err := createFile(name+".gotmpl", "{{.Name}}"); err != nil {
					return codeErr{
						errors.WithMessage(err, fmt.Sprintf("Can't create template file %q", name)),
						1,
					}
				}
			}
			return nil
		},
		Args: cobra.ExactArgs(0),
	}
}

func docCmd(env environ.Values) *cobra.Command {
	return &cobra.Command{
		Use:   "docs",
		Short: "Runs a local webserver serving gnorm documentation.",
		Long: `
Starts a web server running at localhost:8080 that serves docs for this version
of Gnorm.`[1:],
		RunE: func(cmd *cobra.Command, args []string) error {
			return showDocs(env, cmd, args)
		},
	}
}

func createFile(name, contents string) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.WriteString(contents)
	f.Close()
	return err
}

type codeErr struct {
	error
	code int
}

func (c codeErr) Code() int {
	return c.code
}
