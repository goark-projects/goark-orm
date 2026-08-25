package orm

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"time"
)

// IDType 表示 MyBatis-Plus 风格主键生成策略。
type IDType string

const (
	// IDTypeNone 表示未指定主键策略。
	IDTypeNone IDType = ""
	// IDTypeAuto 表示数据库自增或数据库端生成主键。
	IDTypeAuto IDType = "AUTO"
	// IDTypeInput 表示调用方显式传入主键。
	IDTypeInput IDType = "INPUT"
	// IDTypeAssignID 表示 ORM 在插入前分配整数主键。
	IDTypeAssignID IDType = "ASSIGN_ID"
	// IDTypeAssignUUID 表示 ORM 在插入前分配 UUID 字符串主键。
	IDTypeAssignUUID IDType = "ASSIGN_UUID"
)

// IdentifierGenerator 为 ASSIGN_ID 和 ASSIGN_UUID 提供插入前主键。
type IdentifierGenerator interface {
	NextID(ctx context.Context, entity EntityMeta, column ColumnMeta) (any, error)
	NextUUID(ctx context.Context, entity EntityMeta, column ColumnMeta) (string, error)
}

// DefaultIdentifierGenerator 是无外部依赖的默认主键生成器。
type DefaultIdentifierGenerator struct {
	mu         sync.Mutex
	node       int64
	lastMillis int64
	sequence   int64
}

const (
	defaultIdentifierEpochMillis = int64(1704067200000)
	identifierNodeBits           = int64(10)
	identifierSequenceBits       = int64(12)
	identifierNodeMask           = int64((1 << identifierNodeBits) - 1)
	identifierSequenceMask       = int64((1 << identifierSequenceBits) - 1)
	identifierNodeShift          = identifierSequenceBits
	identifierTimestampShift     = identifierNodeBits + identifierSequenceBits
)

// NewDefaultIdentifierGenerator 创建默认主键生成器。
func NewDefaultIdentifierGenerator() *DefaultIdentifierGenerator {
	return &DefaultIdentifierGenerator{node: randomIdentifierNode()}
}

// NextID 生成 Snowflake 风格 64 位整数主键。
func (g *DefaultIdentifierGenerator) NextID(ctx context.Context, entity EntityMeta, column ColumnMeta) (any, error) {
	if ctx == nil {
		return nil, fmt.Errorf("goark-orm: context is nil")
	}
	if g == nil {
		return nil, fmt.Errorf("goark-orm: identifier generator is nil")
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now().UnixMilli()
	if now < g.lastMillis {
		now = g.lastMillis
	}
	if now == g.lastMillis {
		g.sequence = (g.sequence + 1) & identifierSequenceMask
		if g.sequence == 0 {
			next, err := waitNextIdentifierMillis(ctx, now)
			if err != nil {
				return nil, err
			}
			now = next
		}
	} else {
		g.sequence = 0
	}
	g.lastMillis = now
	timestamp := now - defaultIdentifierEpochMillis
	if timestamp < 0 {
		timestamp = now
	}
	return (timestamp << identifierTimestampShift) | ((g.node & identifierNodeMask) << identifierNodeShift) | g.sequence, nil
}

// NextUUID 生成 RFC 4122 Version 4 UUID。
func (g *DefaultIdentifierGenerator) NextUUID(ctx context.Context, entity EntityMeta, column ColumnMeta) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("goark-orm: context is nil")
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("goark-orm: generate uuid failed: %w", err)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(bytes[0:4]),
		binary.BigEndian.Uint16(bytes[4:6]),
		binary.BigEndian.Uint16(bytes[6:8]),
		binary.BigEndian.Uint16(bytes[8:10]),
		bytes[10:16],
	), nil
}

// IdentifierGenerator 返回当前 Session 使用的主键生成器。
func (s *SQLSession) IdentifierGenerator() IdentifierGenerator {
	if s == nil || s.identifierGenerator == nil {
		return NewDefaultIdentifierGenerator()
	}
	return s.identifierGenerator
}

// WithIdentifierGenerator 替换 Session 级主键生成器。
func WithIdentifierGenerator(generator IdentifierGenerator) SQLSessionOption {
	return func(session *SQLSession) error {
		if generator == nil {
			return fmt.Errorf("goark-orm: identifier generator is nil")
		}
		session.identifierGenerator = generator
		return nil
	}
}

// ParseIDType 解析主键策略，兼容下划线和短横线写法。
func ParseIDType(value string) (IDType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	switch normalized {
	case "", "NONE":
		return IDTypeNone, nil
	case string(IDTypeAuto):
		return IDTypeAuto, nil
	case string(IDTypeInput):
		return IDTypeInput, nil
	case string(IDTypeAssignID):
		return IDTypeAssignID, nil
	case string(IDTypeAssignUUID):
		return IDTypeAssignUUID, nil
	default:
		return "", fmt.Errorf("goark-orm: unsupported id-type %q", value)
	}
}

func effectiveColumnIDType(column ColumnMeta) IDType {
	return effectiveColumnIDTypeWithDbConfig(column, DbConfig{})
}

func randomIdentifierNode() int64 {
	var bytes [2]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return int64(binary.BigEndian.Uint16(bytes[:])) & identifierNodeMask
	}
	return time.Now().UnixNano() & identifierNodeMask
}

func waitNextIdentifierMillis(ctx context.Context, current int64) (int64, error) {
	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		now := time.Now().UnixMilli()
		if now > current {
			return now, nil
		}
		time.Sleep(time.Millisecond)
	}
}
