package migrations

import "embed"

const Version int64 = 6

//go:embed *.sql
var Files embed.FS
