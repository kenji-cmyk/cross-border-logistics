package warehousemigration

import _ "embed"

//go:embed 001_create_packages.sql
var SQL string
