package swagger

import "embed"

// @sk-task 118-api-consistency#T3.4: Embed swagger-ui dist and openapi.yaml (AC-008, RQ-010)
//go:embed openapi.yaml
//go:embed swagger-ui/*
var DocsFiles embed.FS
