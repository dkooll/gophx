package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

type BlockProcessor interface {
	ParseAttributes(body *hclsyntax.Body)
	ParseBlocks(body *hclsyntax.Body)
	Validate(t *testing.T, resourceType, path string, schema *SchemaBlock, parentIgnore []string, findings *[]ValidationFinding)
}

type IssueManager interface {
	CreateOrUpdateIssue(findings []ValidationFinding) error
}

type HCLParser interface {
	ParseProviderRequirements(filename string) (map[string]ProviderConfig, error)
	ParseMainFile(filename string) ([]ParsedResource, error)
}

type RepositoryInfoProvider interface {
	GetRepoInfo() (owner, name string)
}

type ValidationFinding struct {
	ResourceType string
	Path         string
	Name         string
	Required     bool
	IsBlock      bool
}

type ProviderConfig struct {
	Source  string
	Version string
}

type ParsedResource struct {
	Type string
	Name string
	data BlockData
}

type BlockData struct {
	properties    map[string]bool
	staticBlocks  map[string]*ParsedBlock
	dynamicBlocks map[string]*ParsedBlock
	ignoreChanges []string
}

type ParsedBlock struct {
	data BlockData
}

// Implement BlockProcessor for BlockData
func NewBlockData() BlockData {
	return BlockData{
		properties:    make(map[string]bool),
		staticBlocks:  make(map[string]*ParsedBlock),
		dynamicBlocks: make(map[string]*ParsedBlock),
		ignoreChanges: []string{},
	}
}

func (bd *BlockData) ParseAttributes(body *hclsyntax.Body) {
	for name := range body.Attributes {
		bd.properties[name] = true
	}
}

func (bd *BlockData) ParseBlocks(body *hclsyntax.Body) {
	for _, block := range body.Blocks {
		switch block.Type {
		case "lifecycle":
			bd.parseLifecycle(block.Body)
		case "dynamic":
			if len(block.Labels) == 1 {
				bd.parseDynamicBlock(block.Body, block.Labels[0])
			}
		default:
			parsed := ParseSyntaxBody(block.Body)
			bd.staticBlocks[block.Type] = parsed
		}
	}
}

func (bd *BlockData) Validate(t *testing.T, resourceType, path string, schema *SchemaBlock, parentIgnore []string, findings *[]ValidationFinding) {
	if schema == nil {
		return
	}

	ignore := append(parentIgnore, bd.ignoreChanges...)
	bd.validateAttributes(t, resourceType, path, schema, ignore, findings)
	bd.validateBlocks(t, resourceType, path, schema, ignore, findings)
}

// Original helper methods
func (bd *BlockData) parseLifecycle(body *hclsyntax.Body) {
	for name, attr := range body.Attributes {
		if name == "ignore_changes" {
			val, _ := attr.Expr.Value(nil)
			bd.ignoreChanges = extractIgnoreChanges(val)
		}
	}
}

func (bd *BlockData) parseDynamicBlock(body *hclsyntax.Body, name string) {
	contentBlock := findContentBlock(body)
	parsed := ParseSyntaxBody(contentBlock)

	if existing := bd.dynamicBlocks[name]; existing != nil {
		mergeBlocks(existing, parsed)
	} else {
		bd.dynamicBlocks[name] = parsed
	}
}

func (bd *BlockData) validateAttributes(t *testing.T, resType, path string, schema *SchemaBlock, ignore []string, findings *[]ValidationFinding) {
	for name, attr := range schema.Attributes {
		if attr.Computed || contains(ignore, name) {
			continue
		}
		if !bd.properties[name] {
			*findings = append(*findings, ValidationFinding{
				ResourceType: resType,
				Path:         path,
				Name:         name,
				Required:     attr.Required,
				IsBlock:      false,
			})
			logMissingAttribute(t, resType, name, path, attr.Required)
		}
	}
}

func (bd *BlockData) validateBlocks(t *testing.T, resType, path string, schema *SchemaBlock, ignore []string, findings *[]ValidationFinding) {
	for name, blockType := range schema.BlockTypes {
		if name == "timeouts" || contains(ignore, name) {
			continue
		}

		static := bd.staticBlocks[name]
		dynamic := bd.dynamicBlocks[name]
		if static == nil && dynamic == nil {
			*findings = append(*findings, ValidationFinding{
				ResourceType: resType,
				Path:         path,
				Name:         name,
				Required:     blockType.MinItems > 0,
				IsBlock:      true,
			})
			logMissingBlock(t, resType, name, path, blockType.MinItems > 0)
			continue
		}

		target := static
		if target == nil {
			target = dynamic
		}

		newPath := fmt.Sprintf("%s.%s", path, name)
		target.data.Validate(t, resType, newPath, blockType.Block, ignore, findings)
	}
}

// HCLParser implementation
type DefaultHCLParser struct{}

func (p *DefaultHCLParser) ParseProviderRequirements(filename string) (map[string]ProviderConfig, error) {
	parser := hclparse.NewParser()
	f, diags := parser.ParseHCLFile(filename)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse error: %v", diags)
	}

	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("invalid body type")
	}

	providers := make(map[string]ProviderConfig)

	for _, blk := range body.Blocks {
		if blk.Type == "terraform" {
			for _, innerBlk := range blk.Body.Blocks {
				if innerBlk.Type == "required_providers" {
					attrs, _ := innerBlk.Body.JustAttributes()
					for name, attr := range attrs {
						val, _ := attr.Expr.Value(nil)
						if val.Type().IsObjectType() {
							pc := ProviderConfig{}
							if sourceVal := val.GetAttr("source"); !sourceVal.IsNull() {
								pc.Source = normalizeSource(sourceVal.AsString())
							}
							if versionVal := val.GetAttr("version"); !versionVal.IsNull() {
								pc.Version = versionVal.AsString()
							}
							providers[name] = pc
						}
					}
				}
			}
		}
	}
	return providers, nil
}

func (p *DefaultHCLParser) ParseMainFile(filename string) ([]ParsedResource, error) {
	parser := hclparse.NewParser()
	f, diags := parser.ParseHCLFile(filename)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse error: %v", diags)
	}

	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("invalid body type")
	}

	var resources []ParsedResource
	for _, blk := range body.Blocks {
		if blk.Type == "resource" && len(blk.Labels) >= 2 {
			parsedBlock := ParseSyntaxBody(blk.Body)
			res := ParsedResource{
				Type: blk.Labels[0],
				Name: blk.Labels[1],
				data: parsedBlock.data,
			}
			resources = append(resources, res)
		}
	}
	return resources, nil
}

// GitHub implementation
type GitHubIssueService struct {
	RepoOwner string
	RepoName  string
	token     string
	Client    *http.Client
}

func (g *GitHubIssueService) CreateOrUpdateIssue(findings []ValidationFinding) error {
	if len(findings) == 0 {
		return nil
	}

	const header = "### \n\n"
	uniqueFindings := make(map[string]ValidationFinding)

	// Deduplicate findings
	for _, f := range findings {
		key := fmt.Sprintf("%s|%s|%s|%v",
			f.ResourceType,
			strings.ReplaceAll(f.Path, "root.", ""),
			f.Name,
			f.IsBlock,
		)
		uniqueFindings[key] = f
	}

	var newBody bytes.Buffer
	fmt.Fprint(&newBody, header)

	// Format findings with line breaks
	for _, f := range uniqueFindings {
		cleanPath := strings.ReplaceAll(f.Path, "root.", "")
		status := "optional"
		if f.Required {
			status = "required"
		}
		itemType := "block"
		if !f.IsBlock {
			itemType = "property"
		}

		fmt.Fprintf(&newBody, "`%s`: Missing %s %s `%s` in %s\n\n", // Note double newline
			f.ResourceType,
			status,
			itemType,
			f.Name,
			cleanPath,
		)
	}

	title := "Generated schema validation"
	issueNumber, existingBody, err := g.findExistingIssue(title)
	if err != nil {
		return err
	}

	finalBody := newBody.String()
	if issueNumber > 0 {
		existingParts := strings.SplitN(existingBody, header, 2)
		if len(existingParts) > 0 {
			finalBody = strings.TrimSpace(existingParts[0]) + "\n\n" + newBody.String()
		}
	}

	if issueNumber > 0 {
		return g.updateIssue(issueNumber, finalBody)
	}
	return g.createIssue(title, finalBody)
}

func (g *GitHubIssueService) findExistingIssue(title string) (int, string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues?state=open", g.RepoOwner, g.RepoName)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "token "+g.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.Client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("GitHub API error: %s", resp.Status)
	}

	var issues []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return 0, "", err
	}

	for _, issue := range issues {
		if issue.Title == title {
			return issue.Number, issue.Body, nil
		}
	}
	return 0, "", nil
}

func (g *GitHubIssueService) updateIssue(issueNumber int, body string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d", g.RepoOwner, g.RepoName, issueNumber)
	payload := struct {
		Body string `json:"body"`
	}{Body: body}

	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PATCH", url, bytes.NewReader(jsonPayload))
	req.Header.Set("Authorization", "token "+g.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.Client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (g *GitHubIssueService) createIssue(title, body string) error {
	payload := struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}{
		Title: title,
		Body:  body,
	}

	jsonPayload, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", g.RepoOwner, g.RepoName)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(jsonPayload))
	req.Header.Set("Authorization", "token "+g.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.Client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// Repository info implementation
type GitRepoInfo struct {
	terraformRoot string
}

func (g *GitRepoInfo) GetRepoInfo() (owner, name string) {
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(g.terraformRoot); err != nil {
		return "", ""
	}

	owner = os.Getenv("GITHUB_REPOSITORY_OWNER")
	name = os.Getenv("GITHUB_REPOSITORY_NAME")
	if owner != "" && name != "" {
		return
	}

	if out, err := exec.Command("git", "remote", "get-url", "origin").Output(); err == nil {
		remote := strings.TrimSpace(string(out))
		return parseGitRemote(remote)
	}

	if config, err := os.ReadFile(".git/config"); err == nil {
		return parseGitConfig(string(config))
	}

	return "", ""
}

// Helper functions
func normalizeSource(source string) string {
	if strings.Contains(source, "/") && !strings.Contains(source, "registry.terraform.io/") {
		return fmt.Sprintf("registry.terraform.io/%s", source)
	}
	return source
}

func extractIgnoreChanges(val cty.Value) []string {
	var changes []string
	if val.Type().IsCollectionType() {
		for it := val.ElementIterator(); it.Next(); {
			_, v := it.Element()
			if v.Type() == cty.String {
				changes = append(changes, v.AsString())
			}
		}
	}
	return changes
}

func findContentBlock(body *hclsyntax.Body) *hclsyntax.Body {
	for _, b := range body.Blocks {
		if b.Type == "content" {
			return b.Body
		}
	}
	return body
}

func mergeBlocks(dest, src *ParsedBlock) {
	for k := range src.data.properties {
		dest.data.properties[k] = true
	}
	for k, v := range src.data.staticBlocks {
		if existing, exists := dest.data.staticBlocks[k]; exists {
			mergeBlocks(existing, v)
		} else {
			dest.data.staticBlocks[k] = v
		}
	}
	for k, v := range src.data.dynamicBlocks {
		if existing, exists := dest.data.dynamicBlocks[k]; exists {
			mergeBlocks(existing, v)
		} else {
			dest.data.dynamicBlocks[k] = v
		}
	}
	dest.data.ignoreChanges = append(dest.data.ignoreChanges, src.data.ignoreChanges...)
}

func logMissingAttribute(t *testing.T, resType, name, path string, required bool) {
	status := "optional"
	if required {
		status = "required"
	}
	cleanPath := strings.ReplaceAll(path, "root.", "")
	t.Logf("%s missing %s property %s in %s", resType, status, name, cleanPath)
}

func logMissingBlock(t *testing.T, resType, name, path string, required bool) {
	status := "optional"
	if required {
		status = "required"
	}
	cleanPath := strings.ReplaceAll(path, "root.", "")
	t.Logf("%s missing %s block %s in %s", resType, status, name, cleanPath)
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func parseGitRemote(remote string) (string, string) {
	if strings.HasPrefix(remote, "https://") {
		parts := strings.Split(remote, "/")
		if len(parts) >= 4 {
			return parts[3], strings.TrimSuffix(parts[4], ".git")
		}
	}

	if strings.HasPrefix(remote, "git@") {
		parts := strings.Split(remote, ":")
		if len(parts) == 2 {
			repoParts := strings.Split(parts[1], "/")
			if len(repoParts) >= 2 {
				return repoParts[0], strings.TrimSuffix(repoParts[1], ".git")
			}
		}
	}

	return "", ""
}

func parseGitConfig(config string) (string, string) {
	lines := strings.Split(config, "\n")
	for i, line := range lines {
		if strings.Contains(line, `[remote "origin"]`) {
			for j := i + 1; j < len(lines); j++ {
				if strings.HasPrefix(lines[j], "\turl = ") {
					return parseGitRemote(strings.TrimPrefix(lines[j], "\turl = "))
				}
			}
		}
	}
	return "", ""
}

func ParseSyntaxBody(body *hclsyntax.Body) *ParsedBlock {
	bd := NewBlockData()
	block := &ParsedBlock{data: bd}
	block.data.ParseAttributes(body)
	block.data.ParseBlocks(body)
	return block
}

// Test function
func TestValidateTerraformSchema(t *testing.T) {
	terraformRoot := os.Getenv("TERRAFORM_ROOT")
	if terraformRoot == "" {
		terraformRoot = filepath.Join("..")
	}

	mainTfPath := filepath.Join(terraformRoot, "main.tf")
	terraformTfPath := filepath.Join(terraformRoot, "terraform.tf")

	if _, err := os.Stat(mainTfPath); err != nil {
		t.Fatalf("No main.tf found at %s: %v", mainTfPath, err)
	}

	var parser HCLParser = &DefaultHCLParser{}
	providers, err := parser.ParseProviderRequirements(terraformTfPath)
	if err != nil {
		t.Fatalf("Failed to parse provider config: %v", err)
	}

	// Cleanup previous Terraform files
	t.Cleanup(func() {
		os.RemoveAll(filepath.Join(terraformRoot, ".terraform"))
		os.Remove(filepath.Join(terraformRoot, "terraform.tfstate"))
		os.Remove(filepath.Join(terraformRoot, ".terraform.lock.hcl"))
	})

	initCmd := exec.CommandContext(context.Background(), "terraform", "init")
	initCmd.Dir = terraformRoot
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("terraform init failed: %v\nOutput: %s", err, string(out))
	}

	schemaCmd := exec.CommandContext(context.Background(), "terraform", "providers", "schema", "-json")
	schemaCmd.Dir = terraformRoot
	schemaBytes, err := schemaCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get schema: %v", err)
	}

	var tfSchema TerraformSchema
	if err := json.Unmarshal(schemaBytes, &tfSchema); err != nil {
		t.Fatalf("Failed to decode schema: %v", err)
	}

	resources, err := parser.ParseMainFile(mainTfPath)
	if err != nil {
		t.Fatalf("Failed to parse main.tf: %v", err)
	}

	var findings []ValidationFinding
	for _, res := range resources {
		providerName := strings.SplitN(res.Type, "_", 2)[0]
		providerConfig, exists := providers[providerName]
		if !exists {
			t.Logf("No provider configured for resource type %s", res.Type)
			continue
		}

		providerSchema := tfSchema.ProviderSchemas[providerConfig.Source]
		if providerSchema == nil {
			t.Logf("No schema found for provider %s (%s)", providerName, providerConfig.Source)
			continue
		}

		resourceSchema := providerSchema.ResourceSchemas[res.Type]
		if resourceSchema == nil {
			continue
		}

		res.data.Validate(t, res.Type, "root", resourceSchema.Block, nil, &findings)
	}

	if ghToken := os.Getenv("GITHUB_TOKEN"); ghToken != "" {
		repoInfo := &GitRepoInfo{terraformRoot: terraformRoot}
		owner, name := repoInfo.GetRepoInfo()
		if owner != "" && name != "" {
			var issueManager IssueManager = &GitHubIssueService{
				RepoOwner: owner,
				RepoName:  name,
				token:     ghToken,
				Client:    &http.Client{Timeout: 10 * time.Second},
			}
			if err := issueManager.CreateOrUpdateIssue(findings); err != nil {
				t.Errorf("Failed to manage GitHub issues: %v", err)
			}
		} else {
			t.Log("Could not determine repository owner/name")
		}
	}
}
