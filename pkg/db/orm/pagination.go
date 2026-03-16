package orm

import (
	"context"
	"fmt"
	"math"
)

// Pagination represents pagination information.
type Pagination struct {
	Page       int    `json:"page"`        // Current page number (1-based)
	PageSize   int    `json:"page_size"`   // Number of items per page
	Total      int64  `json:"total"`       // Total number of items
	TotalPages int    `json:"total_pages"` // Total number of pages
	HasNext    bool   `json:"has_next"`    // Whether there's a next page
	HasPrev    bool   `json:"has_prev"`    // Whether there's a previous page
}

// PaginationResult represents a paginated result set.
type PaginationResult struct {
	Data       interface{}  `json:"data"`       // The actual data
	Pagination *Pagination  `json:"pagination"` // Pagination information
}

// PaginationOptions contains options for pagination.
type PaginationOptions struct {
	Page     int    `json:"page"`     // Page number (1-based, default: 1)
	PageSize int    `json:"page_size"` // Page size (default: 10)
	OrderBy  string `json:"order_by"`  // Order by clause
}

// DefaultPaginationOptions returns default pagination options.
func DefaultPaginationOptions() PaginationOptions {
	return PaginationOptions{
		Page:     1,
		PageSize: 10,
	}
}

// Validate validates and normalizes pagination options.
func (po *PaginationOptions) Validate() {
	if po.Page <= 0 {
		po.Page = 1
	}
	if po.PageSize <= 0 {
		po.PageSize = 10
	}
	if po.PageSize > 1000 {
		po.PageSize = 1000 // Prevent excessive page sizes
	}
}

// CalculateOffset calculates the offset for the current page.
func (po *PaginationOptions) CalculateOffset() int {
	return (po.Page - 1) * po.PageSize
}

// NewPagination creates a new pagination object.
func NewPagination(page, pageSize int, total int64) *Pagination {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	
	return &Pagination{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

// Paginate executes a paginated query using the SelectBuilder.
func (sb *SelectBuilder) Paginate(ctx context.Context, options PaginationOptions) (*PaginationResult, error) {
	options.Validate()
	
	// Clone the builder to avoid modifying the original
	builder := sb.Clone()
	
	// Apply ordering if specified
	if options.OrderBy != "" {
		builder.OrderBy(options.OrderBy)
	}
	
	// Get total count
	countBuilder := sb.Clone()
	countBuilder.Columns("COUNT(*) as total")
	
	var total int64
	countQuery, countArgs := countBuilder.Build()
	err := builder.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count total rows: %w", err)
	}
	
	// Apply pagination
	builder.Limit(options.PageSize)
	builder.Offset(options.CalculateOffset())
	
	// Execute the paginated query
	rows, err := builder.Query(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute paginated query: %w", err)
	}
	defer rows.Close()
	
	// Convert rows to slice of maps
	var data []map[string]interface{}
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		data = append(data, row)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	
	pagination := NewPagination(options.Page, options.PageSize, total)
	
	return &PaginationResult{
		Data:       data,
		Pagination: pagination,
	}, nil
}

// PaginateStruct executes a paginated query and scans results into a slice of structs.
func (sb *SelectBuilder) PaginateStruct(ctx context.Context, options PaginationOptions, dest interface{}) (*PaginationResult, error) {
	options.Validate()
	
	// Clone the builder to avoid modifying the original
	builder := sb.Clone()
	
	// Apply ordering if specified
	if options.OrderBy != "" {
		builder.OrderBy(options.OrderBy)
	}
	
	// Get total count
	countBuilder := sb.Clone()
	countBuilder.Columns("COUNT(*) as total")
	
	var total int64
	countQuery, countArgs := countBuilder.Build()
	err := builder.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count total rows: %w", err)
	}
	
	// Apply pagination
	builder.Limit(options.PageSize)
	builder.Offset(options.CalculateOffset())
	
	// Execute the paginated query
	rows, err := builder.Query(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute paginated query: %w", err)
	}
	defer rows.Close()
	
	// Use sqlx or similar for struct scanning if available
	// For now, we'll return the rows for manual scanning
	pagination := NewPagination(options.Page, options.PageSize, total)
	
	return &PaginationResult{
		Data:       rows,
		Pagination: pagination,
	}, nil
}

// CursorPagination represents cursor-based pagination.
type CursorPagination struct {
	Cursor string `json:"cursor"`      // Cursor for the next page
	Limit  int    `json:"limit"`       // Number of items per page
	HasNext bool  `json:"has_next"`    // Whether there's a next page
}

// CursorPaginationResult represents a cursor-based paginated result.
type CursorPaginationResult struct {
	Data       interface{}       `json:"data"`       // The actual data
	Pagination *CursorPagination `json:"pagination"` // Cursor pagination information
}

// PaginateByCursor executes cursor-based pagination.
func (sb *SelectBuilder) PaginateByCursor(ctx context.Context, cursorColumn string, cursor string, limit int) (*CursorPaginationResult, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 1000 {
		limit = 1000
	}
	
	// Clone the builder to avoid modifying the original
	builder := sb.Clone()
	
	// Apply cursor condition if provided
	if cursor != "" {
		builder.And(fmt.Sprintf("%s > ?", builder.dialect.QuoteIdentifier(cursorColumn)), cursor)
	}
	
	// Apply limit and order by cursor column
	builder.Limit(limit + 1) // Request one extra to check if there's a next page
	builder.OrderBy(fmt.Sprintf("%s ASC", builder.dialect.QuoteIdentifier(cursorColumn)))
	
	// Execute the query
	rows, err := builder.Query(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute cursor paginated query: %w", err)
	}
	defer rows.Close()
	
	// Convert rows to slice of maps
	var data []map[string]interface{}
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	
	rowCount := 0
	for rows.Next() {
		if rowCount >= limit {
			break
		}
		
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		data = append(data, row)
		rowCount++
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	
	// Check if there's a next page
	hasNext := rowCount >= limit
	
	// Get the last cursor value
	var lastCursor string
	if len(data) > 0 {
		if cursorValue, exists := data[len(data)-1][cursorColumn]; exists {
			if str, ok := cursorValue.(string); ok {
				lastCursor = str
			} else {
				lastCursor = fmt.Sprintf("%v", cursorValue)
			}
		}
	}
	
	pagination := &CursorPagination{
		Cursor: lastCursor,
		Limit:  limit,
		HasNext: hasNext,
	}
	
	return &CursorPaginationResult{
		Data:       data,
		Pagination: pagination,
	}, nil
}

// OffsetPagination represents offset-based pagination.
type OffsetPagination struct {
	Offset    int   `json:"offset"`     // Current offset
	Limit     int   `json:"limit"`      // Number of items per page
	Total     int64 `json:"total"`      // Total number of items
	HasNext   bool  `json:"has_next"`   // Whether there's a next page
	HasPrev   bool  `json:"has_prev"`   // Whether there's a previous page
}

// OffsetPaginationResult represents an offset-based paginated result.
type OffsetPaginationResult struct {
	Data       interface{}        `json:"data"`       // The actual data
	Pagination *OffsetPagination `json:"pagination"` // Offset pagination information
}

// PaginateByOffset executes offset-based pagination.
func (sb *SelectBuilder) PaginateByOffset(ctx context.Context, offset, limit int) (*OffsetPaginationResult, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}
	
	// Clone the builder to avoid modifying the original
	builder := sb.Clone()
	
	// Get total count
	countBuilder := sb.Clone()
	countBuilder.Columns("COUNT(*) as total")
	
	var total int64
	countQuery, countArgs := countBuilder.Build()
	err := builder.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count total rows: %w", err)
	}
	
	// Apply offset and limit
	builder.Offset(offset)
	builder.Limit(limit)
	
	// Execute the query
	rows, err := builder.Query(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute offset paginated query: %w", err)
	}
	defer rows.Close()
	
	// Convert rows to slice of maps
	var data []map[string]interface{}
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		data = append(data, row)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	
	pagination := &OffsetPagination{
		Offset:  offset,
		Limit:   limit,
		Total:   total,
		HasNext: int64(offset+limit) < total,
		HasPrev: offset > 0,
	}
	
	return &OffsetPaginationResult{
		Data:       data,
		Pagination: pagination,
	}, nil
}

// PaginateQuery executes a paginated query with custom count and data queries.
func PaginateQuery(ctx context.Context, db DB, countQuery, dataQuery string, countArgs, dataArgs []interface{}, page, pageSize int) (*PaginationResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	
	// Get total count
	var total int64
	err := db.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count total rows: %w", err)
	}
	
	// Apply pagination to data query
	offset := (page - 1) * pageSize
	paginatedDataQuery := fmt.Sprintf("%s LIMIT %d OFFSET %d", dataQuery, pageSize, offset)
	
	// Execute the paginated query
	rows, err := db.Query(ctx, paginatedDataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute paginated query: %w", err)
	}
	defer rows.Close()
	
	// Convert rows to slice of maps
	var data []map[string]interface{}
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		data = append(data, row)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	
	pagination := NewPagination(page, pageSize, total)
	
	return &PaginationResult{
		Data:       data,
		Pagination: pagination,
	}, nil
}
