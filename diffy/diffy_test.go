package diffy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

type TerraformSchema struct {
	ProviderSchemas map[string]*ProviderSchema `json:"provider_schemas"`
}

type ProviderSchema struct {
	ResourceSchemas map[string]*ResourceSchema `json:"resource_schemas"`
}

type ResourceSchema struct {
	Block *SchemaBlock `json:"block"`
}

type SchemaBlock struct {
	Attributes map[string]*SchemaAttribute `json:"attributes"`
	BlockTypes map[string]*SchemaBlockType `json:"block_types"`
}

type SchemaAttribute struct {
	Required bool `json:"required"`
	Optional bool `json:"optional"`
	Computed bool `json:"computed"`
}

type SchemaBlockType struct {
	Nesting  string       `json:"nesting"`
	MinItems int          `json:"min_items"`
	MaxItems int          `json:"max_items"`
	Block    *SchemaBlock `json:"block"`
}

type ParsedResource struct {
	Type          string
	Name          string
	Properties    map[string]bool
	Blocks        map[string]*ParsedBlock
	DynamicBlocks map[string]*ParsedBlock
	IgnoreChanges []string
}

type ParsedBlock struct {
	Properties    map[string]bool
	Blocks        map[string]*ParsedBlock
	DynamicBlocks map[string]*ParsedBlock
	IgnoreChanges []string
}

func TestValidateTerraformSchema(t *testing.T) {
	if _, err := os.Stat("main.tf"); err != nil {
		t.Fatalf("No main.tf found in current directory: %v", err)
	}

	initCmd := exec.CommandContext(context.Background(), "terraform", "init")
	initCmd.Dir = "."
	out, err := initCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("terraform init failed: %v\nOutput: %s", err, string(out))
	}

	schemaBytes, err := getTerraformSchema()
	if err != nil {
		t.Fatalf("Failed to get terraform schema: %v", err)
	}
	var tfSchema TerraformSchema
	if err := json.Unmarshal(schemaBytes, &tfSchema); err != nil {
		t.Fatalf("Failed to decode schema JSON: %v", err)
	}

	azurerm := tfSchema.ProviderSchemas["registry.terraform.io/hashicorp/azurerm"]
	if azurerm == nil {
		t.Fatalf("No azurerm schema found!")
	}

	resources, err := parseFileRawSyntax("main.tf")
	if err != nil {
		t.Fatalf("Failed to parse main.tf: %v", err)
	}
	if len(resources) == 0 {
		t.Fatalf("No resources found in main.tf!")
	}

	for _, r := range resources {
		sch := azurerm.ResourceSchemas[r.Type]
		if sch == nil {
			// Possibly a different provider or custom resource
			continue
		}
		// We'll validate it
		validateResource(t, r, sch.Block)
	}
}

func getTerraformSchema() ([]byte, error) {
	cmd := exec.CommandContext(context.Background(), "terraform", "providers", "schema", "-json")
	return cmd.Output()
}

func parseFileRawSyntax(filename string) ([]ParsedResource, error) {
	parser := hclparse.NewParser()
	f, diags := parser.ParseHCLFile(filename)
	if diags.HasErrors() {
		return nil, fmt.Errorf("ParseHCLFile diags: %v", diags.Error())
	}
	// Convert top-level body to *hclsyntax.Body
	fileBody, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("file body not a *hclsyntax.Body")
	}

	var resources []ParsedResource

	// Each top-level block
	for _, blk := range fileBody.Blocks {
		if blk.Type != "resource" || len(blk.Labels) < 2 {
			continue
		}
		resType := blk.Labels[0]
		resName := blk.Labels[1]

		// Parse the resource's inner body
		resParsed := parseResourceSyntaxBody(blk.Body)

		resources = append(resources, ParsedResource{
			Type:          resType,
			Name:          resName,
			Properties:    resParsed.Properties,
			Blocks:        resParsed.Blocks,
			DynamicBlocks: resParsed.DynamicBlocks,
			IgnoreChanges: resParsed.IgnoreChanges,
		})
	}

	return resources, nil
}

func parseResourceSyntaxBody(b *hclsyntax.Body) *ParsedBlock {
	blockData := &ParsedBlock{
		Properties:    map[string]bool{},
		Blocks:        map[string]*ParsedBlock{},
		DynamicBlocks: map[string]*ParsedBlock{},
		IgnoreChanges: []string{},
	}

	// Attributes at this level
	for name := range b.Attributes {
		blockData.Properties[name] = true
	}

	// Sub-blocks
	for _, sub := range b.Blocks {
		switch sub.Type {
		case "lifecycle":
			parseLifecycleSyntaxBody(sub.Body, blockData)
		case "dynamic":
			if len(sub.Labels) == 1 {
				dynName := sub.Labels[0]
				parseDynamicSyntaxBody(sub.Body, dynName, blockData)
			}
		default:
			blockData.Blocks[sub.Type] = parseBlockSyntaxBody(sub.Body)
		}
	}

	return blockData
}

func parseBlockSyntaxBody(b *hclsyntax.Body) *ParsedBlock {
	blockData := &ParsedBlock{
		Properties:    map[string]bool{},
		Blocks:        map[string]*ParsedBlock{},
		DynamicBlocks: map[string]*ParsedBlock{},
		IgnoreChanges: []string{},
	}

	for name := range b.Attributes {
		blockData.Properties[name] = true
	}
	for _, blk := range b.Blocks {
		switch blk.Type {
		case "lifecycle":
			parseLifecycleSyntaxBody(blk.Body, blockData)
		case "dynamic":
			if len(blk.Labels) == 1 {
				dynName := blk.Labels[0]
				parseDynamicSyntaxBody(blk.Body, dynName, blockData)
			}
		default:
			blockData.Blocks[blk.Type] = parseBlockSyntaxBody(blk.Body)
		}
	}
	return blockData
}

func parseLifecycleSyntaxBody(b *hclsyntax.Body, bd *ParsedBlock) {
	for name, attr := range b.Attributes {
		if name != "ignore_changes" {
			continue
		}
		val, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			continue
		}
		if val.Type().IsListType() || val.Type().IsTupleType() || val.Type().IsSetType() {
			n := val.LengthInt()
			for i := 0; i < n; i++ {
				elem := val.Index(cty.NumberIntVal(int64(i)))
				if elem.Type() == cty.String {
					bd.IgnoreChanges = append(bd.IgnoreChanges, elem.AsString())
				}
			}
		} else if val.Type() == cty.String {
			bd.IgnoreChanges = append(bd.IgnoreChanges, val.AsString())
		}
	}
}

func parseDynamicSyntaxBody(b *hclsyntax.Body, dynName string, parent *ParsedBlock) {
	contentFound := false
	for _, blk := range b.Blocks {
		if blk.Type == "content" {
			contentFound = true
			merged := parseBlockSyntaxBody(blk.Body)
			if parent.DynamicBlocks[dynName] == nil {
				parent.DynamicBlocks[dynName] = &ParsedBlock{
					Properties:    map[string]bool{},
					Blocks:        map[string]*ParsedBlock{},
					DynamicBlocks: map[string]*ParsedBlock{},
					IgnoreChanges: []string{},
				}
			}
			// merge
			for k := range merged.Properties {
				parent.DynamicBlocks[dynName].Properties[k] = true
			}
			for bName, bVal := range merged.Blocks {
				parent.DynamicBlocks[dynName].Blocks[bName] = bVal
			}
			for dName, dVal := range merged.DynamicBlocks {
				parent.DynamicBlocks[dynName].DynamicBlocks[dName] = dVal
			}
			parent.DynamicBlocks[dynName].IgnoreChanges =
				append(parent.DynamicBlocks[dynName].IgnoreChanges, merged.IgnoreChanges...)
		}
	}
	if !contentFound {
		// If there's no content block, parse entire dynamic as fallback
		parent.DynamicBlocks[dynName] = parseBlockSyntaxBody(b)
	}
}

func validateResource(
	t *testing.T,
	r ParsedResource,
	sb *SchemaBlock,
) {
	validateBlockOrResource(
		t,
		r.Type,
		"root", // path
		r.Properties,
		r.Blocks,
		r.DynamicBlocks,
		r.IgnoreChanges,
		sb,
	)
}

// For nested blocks, we do a helper that reuses the same logic
func validateBlock(
	t *testing.T,
	resourceType string,
	path string,
	props map[string]bool,
	blocks map[string]*ParsedBlock,
	dynBlocks map[string]*ParsedBlock,
	ignore []string,
	sb *SchemaBlock,
) {
	validateBlockOrResource(
		t,
		resourceType,
		path,
		props,
		blocks,
		dynBlocks,
		ignore,
		sb,
	)
}

func validateBlockOrResource(
	t *testing.T,
	resourceType string,
	path string,
	props map[string]bool,
	blocks map[string]*ParsedBlock,
	dynBlocks map[string]*ParsedBlock,
	ignore []string,
	sb *SchemaBlock,
) {
	if sb == nil {
		return
	}

	// Check attributes
	for attrName, attrInfo := range sb.Attributes {
		if attrInfo.Computed {
			continue
		}
		if inStringSlice(ignore, attrName) {
			continue
		}
		if !props[attrName] {
			if attrInfo.Required {
				t.Logf("%s missing required property %s in %s",
					resourceType, attrName, path)
			} else {
				t.Logf("%s missing optional property %s in %s",
					resourceType, attrName, path)
			}
		}
	}

	// Check block_types
	for blockName, blockType := range sb.BlockTypes {
		// Skip timouts block
		if blockName == "timeouts" {
			continue
		}

		if inStringSlice(ignore, blockName) {
			continue
		}
		sub := blocks[blockName]
		dyn := dynBlocks[blockName]
		if sub == nil && dyn == nil {
			if blockType.MinItems > 0 {
				t.Logf("%s missing required block %s in %s",
					resourceType, blockName, path)
			} else {
				t.Logf("%s missing optional block %s in %s",
					resourceType, blockName, path)
			}
			continue
		}
		var used *ParsedBlock
		if sub != nil {
			used = sub
		} else {
			used = dyn
		}
		newPath := path + "." + blockName

		validateBlock(
			t,
			resourceType,
			newPath,
			used.Properties,
			used.Blocks,
			used.DynamicBlocks,
			append(ignore, used.IgnoreChanges...),
			blockType.Block,
		)
	}
}

func inStringSlice(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
