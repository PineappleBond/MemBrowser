package token

import "github.com/tiktoken-go/tokenizer"

// Counter 用于计算文本的 Token 数量。
type Counter struct {
	enc tokenizer.Codec
}

// NewCounter 创建一个使用 o200k_base 编码的 Token 计数器。
func NewCounter() (*Counter, error) {
	enc, err := tokenizer.Get(tokenizer.O200kBase)
	if err != nil {
		return nil, err
	}
	return &Counter{enc: enc}, nil
}

// Count 计算单个字符串的 Token 数量。
func (c *Counter) Count(content string) int {
	tokens, _, err := c.enc.Encode(content)
	if err != nil {
		return 0
	}
	return len(tokens)
}

// CountMessages 计算一组消息的 Token 总量。
// 每条消息按 "role: content" 格式拼接后计数。
func (c *Counter) CountMessages(messages []struct{ Role, Content string }) int {
	total := 0
	for _, m := range messages {
		total += c.Count(m.Role + ": " + m.Content)
	}
	return total
}
