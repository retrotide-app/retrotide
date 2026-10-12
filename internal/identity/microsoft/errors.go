package microsoft

import "errors"

var (
	errInvalidTenantID = errors.New("microsoft_tenant_id must be empty or a directory (tenant) ID such as 00000000-0000-0000-0000-000000000000")
	errUnreadableFlag  = errors.New("the claim is not true, false, \"true\", \"1\" or \"false\"")
)
