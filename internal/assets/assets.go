package assets

import "embed"

// Vault contains the portable Agent rules and skills written by learn init.
//
//go:embed all:vault
var Vault embed.FS
