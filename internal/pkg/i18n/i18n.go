// Package i18n 实现简易消息本地化：启动加载 embed 的 toml 文件，按 ctx 中的
// locale 选择最佳匹配，缺翻译 fallback 到默认 locale，再 fallback 到 key 本身。
//
// 我们不引入 nicksnyder/go-i18n（依赖大且不需要复数 / 性别变体），自己 ~80 行
// 搞定 4 国语言 × <200 keys 的需求。
package i18n

import (
	"context"
	"embed"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

//go:embed messages/*.toml
var bundleFS embed.FS

// Bundle 在进程启动时加载一次，之后只读。
type Bundle struct {
	defaultLocale string
	messages      map[string]map[string]string // locale -> "section.key" -> value
}

// New 解析 messages/<locale>.toml 文件列表，construct Bundle。defaultLocale 用
// 作 fallback。
func New(defaultLocale string, locales []string) (*Bundle, error) {
	b := &Bundle{
		defaultLocale: defaultLocale,
		messages:      make(map[string]map[string]string),
	}
	if len(locales) == 0 {
		locales = []string{defaultLocale}
	}
	for _, loc := range locales {
		flat, err := loadLocale(loc)
		if err != nil {
			return nil, fmt.Errorf("i18n load %s: %w", loc, err)
		}
		b.messages[loc] = flat
	}
	if _, ok := b.messages[defaultLocale]; !ok {
		flat, err := loadLocale(defaultLocale)
		if err != nil {
			return nil, fmt.Errorf("i18n default %s: %w", defaultLocale, err)
		}
		b.messages[defaultLocale] = flat
	}
	return b, nil
}

func loadLocale(locale string) (map[string]string, error) {
	raw, err := bundleFS.ReadFile("messages/" + locale + ".toml")
	if err != nil {
		return nil, err
	}
	var nested map[string]map[string]string
	if _, err := toml.Decode(string(raw), &nested); err != nil {
		return nil, err
	}
	flat := make(map[string]string, len(nested)*4)
	for section, kv := range nested {
		for k, v := range kv {
			flat[section+"."+k] = v
		}
	}
	return flat, nil
}

// T 查询本地化字符串。args 非空时按 fmt.Sprintf 格式化。
func (b *Bundle) T(ctx context.Context, key string, args ...any) string {
	loc := LocaleFromContext(ctx)
	if loc == "" {
		loc = b.defaultLocale
	}
	if msg, ok := b.messages[loc][key]; ok {
		return formatMsg(msg, args)
	}
	// fallback to default locale
	if msg, ok := b.messages[b.defaultLocale][key]; ok {
		return formatMsg(msg, args)
	}
	return key
}

// Supported 返回已加载的 locale 列表（排序无关）。
func (b *Bundle) Supported() []string {
	out := make([]string, 0, len(b.messages))
	for k := range b.messages {
		out = append(out, k)
	}
	return out
}

// MatchLocale 用 Accept-Language 头按"quality 排序的简化匹配"挑出最佳 locale。
// 算法：分割 ',', 取每个 token 的 lang 主部分，按顺序看是否在 supported 中（
// 直接比 / 前缀比，如 "zh" 命中 "zh-CN"）。命中即返；都不命中返默认。
func (b *Bundle) MatchLocale(acceptLanguage string) string {
	if acceptLanguage == "" {
		return b.defaultLocale
	}
	tokens := strings.Split(acceptLanguage, ",")
	for _, t := range tokens {
		tag := strings.TrimSpace(strings.SplitN(t, ";", 2)[0])
		if tag == "" {
			continue
		}
		if _, ok := b.messages[tag]; ok {
			return tag
		}
		// prefix match: "zh" -> "zh-CN"
		prefix := strings.SplitN(tag, "-", 2)[0]
		for loc := range b.messages {
			if strings.EqualFold(strings.SplitN(loc, "-", 2)[0], prefix) {
				return loc
			}
		}
	}
	return b.defaultLocale
}

// DefaultLocale 暴露默认 locale，给 mailer 选择目录用。
func (b *Bundle) DefaultLocale() string { return b.defaultLocale }

func formatMsg(msg string, args []any) string {
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}

// ----- context helpers --------------------------------------------------

type ctxKey struct{}

// WithLocale 把 locale 注入 ctx，handler / service 之后用 T(ctx, ...) 即可。
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, ctxKey{}, locale)
}

// LocaleFromContext 取 ctx 中的 locale，未设返 ""。
func LocaleFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		return v
	}
	return ""
}
