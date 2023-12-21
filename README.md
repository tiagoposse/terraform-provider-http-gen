# terraform-provider-oas-codegen

Generate a working terraform provider from an openapi spec. It will use:
- https://developer.hashicorp.com/terraform/plugin/code-generation/openapi-generator
- https://developer.hashicorp.com/terraform/plugin/code-generation/framework-generator
- https://github.com/deepmap/oapi-codegen

In addition, it will add custom code to call the generated clients.

To get started, create a configuration file:
```
generator:
  base: # base folder where code will exist. If this is 'internal' in a repo called 'example', code will be generated under 'example/internal'
  oasPath: # path to the openapi spec to use as reference

oapi-codegen: {} # config file openapi codegen. Sane defaults are generated. For advanced use check: https://github.com/deepmap/oapi-codegen

terraform: # tf codegen config, for reference check: https://developer.hashicorp.com/terraform/plugin/code-generation/openapi-generator
  provider:
    name: # provider nome
    schema_ref: # openapi schema for the provider, e.g.: '#/components/schemas/Provider'

  resources:
    example:
      create:
        path: /example
        method: POST
      update:
        path: /example/{id}
        method: PATCH
      read:
        path: /example/{id}
        method: GET
      delete:
        path: /example/{id}
        method: DELETE

  data_sources:
    example:
      read:
        path: /example/{id}
        method: GET
```

```
mkdir -p example/generate
go mod init github.com/example/repo

cat << 'EOF' > generate/generate.go
package generate

//go:generate go run github.com/tiagoposse/terraform-provider-oas-codegen/cmd/tfprovider-oas-gen generate
'EOF'

```
