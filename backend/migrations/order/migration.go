package ordermigration

import _ "embed"

//go:embed 001_create_orders.sql
var SQL string
