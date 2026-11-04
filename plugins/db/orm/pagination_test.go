package orm

import (
	"testing"
)

// ==================== PaginationOptions ====================

func TestDefaultPaginationOptions(t *testing.T) {
	opts := DefaultPaginationOptions()
	if opts.Page != 1 {
		t.Errorf("Page = %d, want 1", opts.Page)
	}
	if opts.PageSize != 10 {
		t.Errorf("PageSize = %d, want 10", opts.PageSize)
	}
}

func TestPaginationOptions_Validate(t *testing.T) {
	tests := []struct {
		name     string
		input    PaginationOptions
		wantPage int
		wantSize int
	}{
		{"zero_values", PaginationOptions{}, 1, 10},
		{"negative_values", PaginationOptions{Page: -1, PageSize: -5}, 1, 10},
		{"too_large_size", PaginationOptions{Page: 1, PageSize: 2000}, 1, 1000},
		{"valid", PaginationOptions{Page: 3, PageSize: 20}, 3, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.input
			opts.Validate()
			if opts.Page != tt.wantPage {
				t.Errorf("Page = %d, want %d", opts.Page, tt.wantPage)
			}
			if opts.PageSize != tt.wantSize {
				t.Errorf("PageSize = %d, want %d", opts.PageSize, tt.wantSize)
			}
		})
	}
}

func TestPaginationOptions_CalculateOffset(t *testing.T) {
	tests := []struct {
		page     int
		pageSize int
		want     int
	}{
		{1, 10, 0},
		{2, 10, 10},
		{3, 20, 40},
		{0, 10, -10}, // page 0 gives negative offset (caller should Validate first)
	}
	for _, tt := range tests {
		opts := PaginationOptions{Page: tt.page, PageSize: tt.pageSize}
		got := opts.CalculateOffset()
		if got != tt.want {
			t.Errorf("CalculateOffset(page=%d, size=%d) = %d, want %d", tt.page, tt.pageSize, got, tt.want)
		}
	}
}

// ==================== NewPagination ====================

func TestNewPagination(t *testing.T) {
	tests := []struct {
		name       string
		page       int
		pageSize   int
		total      int64
		wantPages  int
		wantNext   bool
		wantPrev   bool
	}{
		{"first_page", 1, 10, 100, 10, true, false},
		{"middle_page", 5, 10, 100, 10, true, true},
		{"last_page", 10, 10, 100, 10, false, true},
		{"zero_page", 0, 10, 50, 5, true, false},   // page 0 → 1
		{"zero_size", 1, 0, 50, 5, true, false},     // size 0 → 10
		{"empty_result", 1, 10, 0, 0, false, false},
		{"single_page", 1, 10, 5, 1, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPagination(tt.page, tt.pageSize, tt.total)
			if p.TotalPages != tt.wantPages {
				t.Errorf("TotalPages = %d, want %d", p.TotalPages, tt.wantPages)
			}
			if p.HasNext != tt.wantNext {
				t.Errorf("HasNext = %v, want %v", p.HasNext, tt.wantNext)
			}
			if p.HasPrev != tt.wantPrev {
				t.Errorf("HasPrev = %v, want %v", p.HasPrev, tt.wantPrev)
			}
		})
	}
}

// ==================== Pagination struct fields ====================

func TestPaginationStruct(t *testing.T) {
	p := &Pagination{
		Page:       2,
		PageSize:   20,
		Total:      100,
		TotalPages: 5,
		HasNext:    true,
		HasPrev:    true,
	}
	if p.Page != 2 {
		t.Error("Page mismatch")
	}
	if p.PageSize != 20 {
		t.Error("PageSize mismatch")
	}
	if p.Total != 100 {
		t.Error("Total mismatch")
	}
}

// ==================== PaginationResult struct ====================

func TestPaginationResultStruct(t *testing.T) {
	pr := &PaginationResult{
		Data:       []string{"a", "b"},
		Pagination: NewPagination(1, 10, 2),
	}
	if pr.Pagination.Total != 2 {
		t.Error("PaginationResult pagination total mismatch")
	}
}

// ==================== CursorPagination struct ====================

func TestCursorPaginationStruct(t *testing.T) {
	cp := &CursorPagination{
		Cursor:  "abc123",
		Limit:   10,
		HasNext: true,
	}
	if cp.Cursor != "abc123" {
		t.Error("Cursor mismatch")
	}
	if cp.Limit != 10 {
		t.Error("Limit mismatch")
	}
	if !cp.HasNext {
		t.Error("HasNext should be true")
	}
}

// ==================== OffsetPagination struct ====================

func TestOffsetPaginationStruct(t *testing.T) {
	op := &OffsetPagination{
		Offset:  20,
		Limit:   10,
		Total:   100,
		HasNext: true,
		HasPrev: true,
	}
	if op.Offset != 20 {
		t.Error("Offset mismatch")
	}
	if op.Total != 100 {
		t.Error("Total mismatch")
	}
}
