package migrations

import "embed"

const Version int64 = 7

//go:embed *.sql
var Files embed.FS
