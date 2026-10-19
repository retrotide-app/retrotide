package dto

type (
	ClientErrorReportRequest struct {
		Name           string `json:"name"`
		Message        string `json:"message"`
		Stack          string `json:"stack"`
		ComponentStack string `json:"component_stack"`
		Source         string `json:"source"`
		Route          string `json:"route"`
		Version        string `json:"version"`
		Sequence       int    `json:"sequence"`
	}
)
