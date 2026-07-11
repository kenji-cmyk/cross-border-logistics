package quotationmigration

import _ "embed"

//go:embed 001_create_quotations.sql
var SQL string
