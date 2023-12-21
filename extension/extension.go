package extension

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"entgo.io/ent/entc/gen"
	"github.com/hashicorp/terraform-plugin-codegen-spec/spec"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

var (
	ProviderTemplate = parseT("templates/provider.tmpl")
	DataTemplate     = parseT("templates/data.tmpl")
	ResourceTemplate = parseT("templates/resource.tmpl")

	TemplateFuncs = template.FuncMap{}

	//go:embed templates/*
	_templates embed.FS
)

type GeneratorConfig struct {
	Base        string `yaml:"base"`
	OpenApiSpec string `yaml:"oasPath"`
}

type Config struct {
	Generator GeneratorConfig `yaml:"generator"`
	OpenAPI   map[string]any  `yaml:"oapi-codegen"`
	Terraform map[string]any  `yaml:"terraform"`
}

// oapi-codegen:
//   package: clients
//   generate:
//     models: true
//     client: true
//   # output: ../internal/clients/clients.gen.go
//   output-options:
//     exclude-tags:
//       - x-tf-ignore

// terraform:
type TerraformExtension struct {
	PackageName string
	RepoPath    string
	ConfigPath  string
}

// DisallowTypeName ensures there is no ent.Schema with the given name in the graph.
func (a *TerraformExtension) Generate() error {
	content, err := os.ReadFile(a.ConfigPath)
	if err != nil {
		return err
	}

	var conf Config
	if err := yaml.Unmarshal(content, &conf); err != nil {
		return err
	}

	if _, ok := conf.OpenAPI["output"]; !ok {
		conf.OpenAPI["output"] = filepath.Join(a.RepoPath, conf.Generator.Base, "clients", "clients.gen.go")
	}
	if err := os.MkdirAll(filepath.Dir(conf.OpenAPI["output"].(string)), os.ModePerm); err != nil {
		return fmt.Errorf("creating output dir for openapi clients: %w", err)
	}

	if _, ok := conf.OpenAPI["generate"]; !ok {
		conf.OpenAPI["generate"] = map[string]bool{
			"client": true,
			"models": true,
		}
	}

	if _, ok := conf.OpenAPI["package"]; !ok {
		conf.OpenAPI["package"] = "clients"
	}

	if val, ok := conf.OpenAPI["output-options"]; !ok {
		conf.OpenAPI["output-options"] = map[string][]string{
			"exclude-tags": {
				"x-tf-ignore",
			},
		}
	} else {
		if excluded, ok := val.(map[string]any)["exclude-tags"]; !ok {
			conf.OpenAPI["output-options"].(map[string]any)["exclude-tags"] = []string{
				"x-tf-ignore",
			}
		} else if slices.Index(excluded.([]string), "x-tf-ignore") == -1 {
			excluded = append(excluded.([]string), "x-tf-ignore")
			conf.OpenAPI["output-options"].(map[string]any)["exclude-tags"] = excluded
		}
	}

	confDir := filepath.Dir(a.ConfigPath)
	oasConfPath := filepath.Join(confDir, "oas.yaml")
	tfConfPath := filepath.Join(confDir, "tfconfig.yaml")
	provSpecPath := filepath.Join(confDir, "provider-spec.json")
	tfCodePath := filepath.Join(a.RepoPath, conf.Generator.Base, "provider")

	tfcontent, err := yaml.Marshal(conf.Terraform)
	if err != nil {
		return err
	}

	if err := os.WriteFile(tfConfPath, tfcontent, os.ModePerm); err != nil {
		return err
	}

	oascontent, err := yaml.Marshal(conf.OpenAPI)
	if err != nil {
		return err
	}
	if err := os.WriteFile(oasConfPath, oascontent, os.ModePerm); err != nil {
		return err
	}

	cmd := exec.Command(
		"go", "run",
		"github.com/hashicorp/terraform-plugin-codegen-openapi/cmd/tfplugingen-openapi@latest",
		"generate",
		"--config", tfConfPath,
		"--output", provSpecPath,
		conf.Generator.OpenApiSpec,
	)
	out, err := cmd.CombinedOutput()
	fmt.Println(string(out))
	if err != nil {
		return fmt.Errorf("generating provider spec: %w", err)
	}

	cmd = exec.Command(
		"go", "run",
		"github.com/hashicorp/terraform-plugin-codegen-framework/cmd/tfplugingen-framework@latest",
		"generate", "all",
		"--input", provSpecPath,
		"--output", tfCodePath,
	)
	out, err = cmd.CombinedOutput()
	fmt.Println(string(out))
	if err != nil {
		return fmt.Errorf("generating tf code from provider spec: %w", err)
	}

	cmd = exec.Command(
		"go", "run",
		"github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest",
		"--config", oasConfPath,
		conf.Generator.OpenApiSpec,
	)
	out, err = cmd.CombinedOutput()
	fmt.Println(string(out))
	if err != nil {
		return fmt.Errorf("generating openapi clients: %w", err)
	}

	var inputSpec spec.Specification
	content, err = os.ReadFile(provSpecPath)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(content, &inputSpec); err != nil {
		return err
	}

	resources := make(map[string]string)
	datasources := make(map[string]string)

	conf.OpenAPI["output"] = filepath.Join(a.RepoPath, conf.Generator.Base, "clients", "clients.gen.go")
	clients := fmt.Sprintf(
		"%s/%s",
		a.PackageName,
		strings.TrimPrefix(filepath.Dir(conf.OpenAPI["output"].(string)), fmt.Sprintf("%s%s", a.RepoPath, string(filepath.Separator))),
	)

	for _, resource := range inputSpec.Resources {
		capitalized_resource_name := cases.Title(language.English, cases.Compact).String(resource.Name)
		resource_data := map[string]string{
			"Name":        capitalized_resource_name,
			"PackageName": fmt.Sprintf("resource_%s", resource.Name),
			"Clients":     clients,
		}

		datasource_data := map[string]string{
			"Name":        capitalized_resource_name,
			"PackageName": fmt.Sprintf("datasource_%s", resource.Name),
			"Clients":     clients,
		}
		resources[resource_data["Name"]] = resource_data["PackageName"]
		datasources[datasource_data["Name"]] = datasource_data["PackageName"]

		if err := executeTemplate(
			ResourceTemplate,
			"entform/resource",
			filepath.Join(tfCodePath, fmt.Sprintf("resource_%s", resource.Name), fmt.Sprintf("%s_resource_impl_gen.go", resource.Name)),
			resource_data,
		); err != nil {
			return err
		}

		if err := executeTemplate(
			DataTemplate,
			"entform/data",
			filepath.Join(tfCodePath, fmt.Sprintf("datasource_%s", resource.Name), fmt.Sprintf("%s_data_impl_gen.go", resource.Name)),
			datasource_data,
		); err != nil {
			return err
		}
	}

	if err := executeTemplate(
		ProviderTemplate,
		"entform/provider",
		filepath.Join(tfCodePath, fmt.Sprintf("provider_%s", strings.ToLower(inputSpec.Provider.Name)), "provider_impl_gen.go"),
		map[string]any{
			"Resources":      resources,
			"DataSources":    datasources,
			"Name":           cases.Title(language.English, cases.Compact).String(inputSpec.Provider.Name),
			"PackageName":    "provider_" + strings.ToLower(inputSpec.Provider.Name),
			"ClientsPackage": fmt.Sprintf("%s/%s/provider", a.PackageName, conf.Generator.Base),
			"Clients":        clients,
		},
	); err != nil {
		return err
	}

	return nil
}

func executeTemplate(tmpl *gen.Template, name, target string, params any) error {
	f, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("opening file for write: %w", err)
	}

	defer f.Close()
	return tmpl.ExecuteTemplate(f, name, params)
}

func parseT(path string) *gen.Template {
	return gen.MustParse(gen.NewTemplate(path).
		Funcs(TemplateFuncs).
		ParseFS(_templates, path))
}
