package migrations

import "embed"

const Version int64 = 4

//go:embed *.sql
var Files embed.FS
