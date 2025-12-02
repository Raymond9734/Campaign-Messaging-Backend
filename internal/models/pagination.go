package models

// PaginationResult holds pagination metadata
type PaginationResult struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalCount int64 `json:"total_count"`
	TotalPages int   `json:"total_pages"`
}

// NewPaginationResult creates a pagination result
func NewPaginationResult(page, pageSize int, totalCount int64) PaginationResult {
	totalPages := int(totalCount) / pageSize
	if int(totalCount)%pageSize > 0 {
		totalPages++
	}

	return PaginationResult{
		Page:       page,
		PageSize:   pageSize,
		TotalCount: totalCount,
		TotalPages: totalPages,
	}
}

// ValidateAndSetDefaults validates pagination parameters and sets defaults
func ValidateAndSetDefaults(page, pageSize *int) {
	if *page < 1 {
		*page = 1
	}
	if *pageSize < 1 {
		*pageSize = 20
	}
	if *pageSize > 100 {
		*pageSize = 100
	}
}

// CalculateOffset calculates the SQL offset for pagination
func CalculateOffset(page, pageSize int) int {
	return (page - 1) * pageSize
}
