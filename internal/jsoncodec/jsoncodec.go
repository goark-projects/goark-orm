// Package jsoncodec 集中封装 ORM 内部 JSON 编解码能力。
package jsoncodec

import (
	"io"

	"github.com/bytedance/sonic"
)

var (
	fastJSON = sonic.ConfigFastest
	// 配置文件属于外部输入，必须保留未知字段 fail-fast 语义。
	strictJSON = sonic.Config{
		DisallowUnknownFields: true,
		CopyString:            true,
		ValidateString:        true,
	}.Froze()
)

// Marshal 使用 Sonic 快速序列化 JSON。
func Marshal(value any) ([]byte, error) {
	return fastJSON.Marshal(value)
}

// Unmarshal 使用 Sonic 快速反序列化 JSON 字节。
func Unmarshal(data []byte, target any) error {
	return fastJSON.Unmarshal(data, target)
}

// UnmarshalString 使用 Sonic 快速反序列化 JSON 字符串。
func UnmarshalString(value string, target any) error {
	return fastJSON.UnmarshalFromString(value, target)
}

// DecodeStrict 从流中解码 JSON，并拒绝目标结构体未声明的字段。
func DecodeStrict(reader io.Reader, target any) error {
	decoder := strictJSON.NewDecoder(reader)
	return decoder.Decode(target)
}

// Valid 判断字节内容是否为合法 JSON。
func Valid(data []byte) bool {
	return fastJSON.Valid(data)
}
