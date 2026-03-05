package validator

import (
	"fmt"
	"os"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

type Validator struct {
	schema *jsonschema.Schema
}

func New(schemaPath string) (*Validator, error) {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", strings.NewReader(string(data))); err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}

	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}

	return &Validator{schema: schema}, nil
}

// Validate returns nil if valid, or a descriptive error suitable for DLQ context.
func (v *Validator) Validate(doc any) error {
	if err := v.schema.Validate(doc); err != nil {
		return fmt.Errorf("schema validation: %w", err)
	}
	return nil
}
