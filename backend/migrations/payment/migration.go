package paymentmigration

import _ "embed"

//go:embed 001_create_payments.sql
var SQL string
