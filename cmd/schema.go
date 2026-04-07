package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deca-org/deca/internal/config"
	"github.com/deca-org/deca/internal/schema"
	"github.com/deca-org/deca/internal/ui"
	"github.com/spf13/cobra"
)

var schemaOutput string
var schemaInject bool

// SchemaCmd generates the JSON Schema for deca.toml
var SchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Generate JSON Schema for deca.toml",
	Long: `Generate a JSON Schema (draft-07) for the deca.toml configuration file.

The schema enables auto-completion and validation in editors that support
TOML LSPs such as tombi (https://github.com/tombi-toml/tombi).

Examples:
  # Print schema to stdout
  deca schema

  # Write schema to a file
  deca schema --output deca.schema.json

  # Write schema and inject $schema key into deca.toml
  deca schema --inject

  # Write schema to a custom path and inject reference
  deca schema --output ~/.config/deca/deca.schema.json --inject`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Determine the output path for the schema file.
		outPath := schemaOutput
		if schemaInject && outPath == "" {
			// Default schema path next to the config file.
			outPath = filepath.Join(config.DefaultConfigDir(), "deca.schema.json")
		}

		if outPath != "" {
			if err := schema.WriteSchema(outPath); err != nil {
				return fmt.Errorf("failed to write schema: %w", err)
			}
			ui.Success.Printf("Schema written to: %s\n", outPath)
		} else {
			schemaBytes, err := schema.GenerateSchema()
			if err != nil {
				return fmt.Errorf("failed to generate schema: %w", err)
			}
			// Print to stdout.
			fmt.Println(string(schemaBytes))
		}

		if schemaInject {
			configPath := getConfigPath()
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				ui.Warning.Printf("Config file not found: %s — skipping injection\n", configPath)
				return nil
			}
			if err := schema.InjectSchemaReference(configPath, outPath); err != nil {
				return fmt.Errorf("failed to inject schema reference: %w", err)
			}
			ui.Success.Printf("Injected $schema reference into: %s\n", configPath)
		}

		return nil
	},
}

func init() {
	SchemaCmd.Flags().StringVarP(&schemaOutput, "output", "o", "", "Write schema to this file (default: stdout)")
	SchemaCmd.Flags().BoolVar(&schemaInject, "inject", false, `Add "$schema" key to deca.toml pointing to the schema file`)

	RootCmd.AddCommand(SchemaCmd)
}
