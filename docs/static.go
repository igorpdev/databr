package docs

import "embed"

//go:embed landing.html scalar.html openapi.yaml
var Static embed.FS
