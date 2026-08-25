package orm

// PageRequest 描述分页查询输入。
type PageRequest struct {
	Current     int64
	Size        int64
	SearchCount bool
	MaxLimit    int64
}

// Page 描述分页查询结果。
type Page[T any] struct {
	Records []T
	Total   int64
	Size    int64
	Current int64
	Pages   int64
}

// NewPageRequest 创建分页请求，页码从 1 开始。
func NewPageRequest(current int64, size int64) PageRequest {
	return PageRequest{
		Current:     current,
		Size:        size,
		SearchCount: true,
	}
}

func (p PageRequest) normalized() PageRequest {
	if p.Current < 1 {
		p.Current = 1
	}
	if p.Size == 0 {
		p.Size = 10
	}
	if p.MaxLimit > 0 && p.Size > p.MaxLimit {
		p.Size = p.MaxLimit
	}
	return p
}

func (p PageRequest) offset() int64 {
	p = p.normalized()
	if p.Size < 0 {
		return 0
	}
	return (p.Current - 1) * p.Size
}

func pageCount(total int64, size int64) int64 {
	if total <= 0 || size <= 0 {
		return 0
	}
	pages := total / size
	if total%size != 0 {
		pages++
	}
	return pages
}
