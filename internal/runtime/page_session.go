package runtime

import (
	"context"
	"fmt"
	"strings"
)

// PageQueryResult 描述分页查询除记录外的统计信息。
type PageQueryResult struct {
	Total   int64
	Size    int64
	Current int64
	Pages   int64
}

// PageQuerySession 描述支持直接分页查询的 Session。
type PageQuerySession interface {
	QueryPage(ctx context.Context, statement string, args NamedArgs, page PageRequest, dest any) (PageQueryResult, error)
}

// QueryPage 执行生成 Mapper 使用的分页查询。
func QueryPage[T any](ctx context.Context, session Session, statement string, args NamedArgs, page PageRequest) (Page[T], error) {
	if session == nil {
		return Page[T]{}, fmt.Errorf("goark-orm: session is nil")
	}
	page = page.normalized()
	var records []T
	paged, ok := session.(PageQuerySession)
	if !ok {
		if page.SearchCount {
			return Page[T]{}, fmt.Errorf("goark-orm: session does not support page query")
		}
		if err := session.Query(WithPageRequest(ctx, page), statement, args, &records); err != nil {
			return Page[T]{}, err
		}
		return Page[T]{
			Records: records,
			Size:    page.Size,
			Current: page.Current,
		}, nil
	}
	result, err := paged.QueryPage(ctx, statement, args, page, &records)
	if err != nil {
		return Page[T]{}, err
	}
	return Page[T]{
		Records: records,
		Total:   result.Total,
		Size:    result.Size,
		Current: result.Current,
		Pages:   result.Pages,
	}, nil
}

// QueryPage 执行分页查询并把记录扫描到 dest。
func (s *SQLSession) QueryPage(ctx context.Context, statement string, args NamedArgs, page PageRequest, dest any) (PageQueryResult, error) {
	if ctx == nil {
		return PageQueryResult{}, fmt.Errorf("goark-orm: context is nil")
	}
	meta, err := s.lookupStatement(statement)
	if err != nil {
		return PageQueryResult{}, err
	}
	return s.QueryPageStatement(ctx, meta, args, page, dest)
}

// QueryPageStatement 基于 StatementMeta 执行分页查询。
func (s *SQLSession) QueryPageStatement(ctx context.Context, meta StatementMeta, args NamedArgs, page PageRequest, dest any) (PageQueryResult, error) {
	if ctx == nil {
		return PageQueryResult{}, fmt.Errorf("goark-orm: context is nil")
	}
	if meta.Command != StatementCommandSelect {
		return PageQueryResult{}, fmt.Errorf("goark-orm: statement %s is %s, not select", meta.FullName, meta.Command)
	}
	page = page.normalized()
	result := PageQueryResult{
		Size:    page.Size,
		Current: page.Current,
	}
	runtime, err := s.statementHandler.Prepare(withPaginationDisabled(ctx), meta, args)
	if err != nil {
		return PageQueryResult{}, err
	}
	if err := s.flushStatementCaches(ctx, meta); err != nil {
		return PageQueryResult{}, err
	}
	if page.SearchCount {
		countSQL := "SELECT COUNT(*) FROM (" + countSQLBase(runtime.SQL) + ") goark_orm_count"
		var total int64
		if err := s.queryCompiledOne(ctx, runtime.Meta, runtime.Dialect, countSQL, runtime.Args, runtime.CacheKey, &total); err != nil {
			return PageQueryResult{}, err
		}
		result.Total = total
		result.Pages = pageCount(total, page.Size)
	}
	listSQL := runtime.SQL
	listArgs := copyNamedArgs(runtime.Args)
	if page.Size >= 0 {
		limitName := nextSQLArgName(listArgs, "__goark_orm_page_limit")
		listArgs[limitName] = page.Size
		offsetName := nextSQLArgName(listArgs, "__goark_orm_page_offset")
		listArgs[offsetName] = page.offset()
		listSQL = limitOffsetSQL(runtime.Dialect, listSQL, "#{"+limitName+"}", "#{"+offsetName+"}")
	}
	if err := s.queryCompiledRows(ctx, runtime.Meta, runtime.Dialect, listSQL, listArgs, runtime.CacheKey, dest); err != nil {
		return PageQueryResult{}, err
	}
	return result, nil
}

func (s *SQLSession) queryCompiledOne(ctx context.Context, meta StatementMeta, dialect Dialect, sqlText string, args NamedArgs, providerCacheKey string, dest any) error {
	compiled, err := s.statementHandler.CompileText(ctx, meta, dialect, sqlText, args)
	if err != nil {
		return err
	}
	compiled.CacheKey = strings.TrimSpace(providerCacheKey)
	cacheKey, useCache := s.queryCacheKey(meta, compiled)
	if useCache {
		if hit, err := s.getLocalCache(cacheKey, dest); err != nil || hit {
			return err
		}
		if hit, err := s.getSecondLevelCache(ctx, meta, cacheKey, dest); err != nil || hit {
			return err
		}
		defer s.releaseSecondLevelCacheMiss(ctx, meta, cacheKey)
	}
	rows, err := s.querySQL(ctx, meta, compiled)
	if err != nil {
		return err
	}
	scanErr := s.resultSetHandler.ScanOne(ctx, rows, meta, dest)
	closeErr := rows.Close()
	if scanErr != nil {
		return scanErr
	}
	if closeErr != nil {
		return closeErr
	}
	if useCache {
		if err := s.putLocalCache(cacheKey, dest); err != nil {
			return err
		}
		return s.putSecondLevelCache(ctx, meta, cacheKey, dest)
	}
	return nil
}

func (s *SQLSession) queryCompiledRows(ctx context.Context, meta StatementMeta, dialect Dialect, sqlText string, args NamedArgs, providerCacheKey string, dest any) error {
	compiled, err := s.statementHandler.CompileText(ctx, meta, dialect, sqlText, args)
	if err != nil {
		return err
	}
	compiled.CacheKey = strings.TrimSpace(providerCacheKey)
	cacheKey, useCache := s.queryCacheKey(meta, compiled)
	if useCache {
		if hit, err := s.getLocalCache(cacheKey, dest); err != nil || hit {
			return err
		}
		if hit, err := s.getSecondLevelCache(ctx, meta, cacheKey, dest); err != nil || hit {
			return err
		}
		defer s.releaseSecondLevelCacheMiss(ctx, meta, cacheKey)
	}
	rows, err := s.querySQL(ctx, meta, compiled)
	if err != nil {
		return err
	}
	scanErr := s.resultSetHandler.ScanRows(ctx, rows, meta, dest)
	closeErr := rows.Close()
	if scanErr != nil {
		return scanErr
	}
	if closeErr != nil {
		return closeErr
	}
	if useCache {
		if err := s.putLocalCache(cacheKey, dest); err != nil {
			return err
		}
		return s.putSecondLevelCache(ctx, meta, cacheKey, dest)
	}
	return nil
}

func (s *SQLSession) compileSQLText(ctx context.Context, meta StatementMeta, dialect Dialect, sqlText string, args NamedArgs) (CompiledSQL, error) {
	boundArgs, err := s.parameterHandler.Bind(ctx, meta, args)
	if err != nil {
		return CompiledSQL{}, fmt.Errorf("goark-orm: bind statement %s failed: %w", meta.FullName, err)
	}
	compiled, err := CompileSQLContext(ctx, sqlText, boundArgs, dialect)
	if err != nil {
		return CompiledSQL{}, fmt.Errorf("goark-orm: compile statement %s failed: %w", meta.FullName, err)
	}
	return compiled, nil
}

func countSQLBase(query string) string {
	head, _ := splitSQLTail(query)
	return head
}

// QueryPage 会先刷新待执行写语句，再执行分页查询。
func (s *BatchSession) QueryPage(ctx context.Context, statement string, args NamedArgs, page PageRequest, dest any) (PageQueryResult, error) {
	if _, err := s.Flush(ctx); err != nil {
		return PageQueryResult{}, err
	}
	paged, ok := s.session.(PageQuerySession)
	if !ok {
		return PageQueryResult{}, fmt.Errorf("goark-orm: batch delegate does not support page query")
	}
	return paged.QueryPage(ctx, statement, args, page, dest)
}

// QueryPage 执行事务内分页查询。
func (s *TxSession) QueryPage(ctx context.Context, statement string, args NamedArgs, page PageRequest, dest any) (PageQueryResult, error) {
	if err := s.ensureActive(); err != nil {
		return PageQueryResult{}, err
	}
	return s.session.QueryPage(ctx, statement, args, page, dest)
}
