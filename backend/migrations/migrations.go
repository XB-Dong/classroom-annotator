package migrations

import "embed"

const Version int64 = 5

//go:embed *.sql
var Files embed.FS
