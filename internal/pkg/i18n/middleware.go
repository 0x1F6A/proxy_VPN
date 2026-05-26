// Gin middleware：从 Accept-Language 头算出最佳 locale 注入 ctx + 暴露给 handler。
package i18n

import (
	"github.com/gin-gonic/gin"
)

// Middleware 把 Accept-Language → locale 写入 c.Request.Context() 和 c.Set("locale")。
// handler 取 ctx 用 LocaleFromContext，模板用 c.GetString("locale")。
func Middleware(b *Bundle) gin.HandlerFunc {
	return func(c *gin.Context) {
		al := c.GetHeader("Accept-Language")
		loc := b.MatchLocale(al)
		c.Set("locale", loc)
		c.Request = c.Request.WithContext(WithLocale(c.Request.Context(), loc))
		c.Next()
	}
}
