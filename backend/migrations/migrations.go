package migrations

import "embed"

const Version int64 = 8

//go:embed *.sql
var Files embed.FS
