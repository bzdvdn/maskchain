package ui

import "embed"

// @sk-task 41-profiles-ui#T1.2: Embed UI dist files (AC-001)
//
//go:embed dist/*
var DistFiles embed.FS
