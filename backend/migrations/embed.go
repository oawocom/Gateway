package migrations

import "embed"

//go:embed master/*.sql tenant/*.sql
var Files embed.FS
