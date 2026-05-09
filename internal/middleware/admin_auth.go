package middleware

import (
	"cleanmark/pkg/response"
	"strings"

	"github.com/gin-gonic/gin"
)

func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "管理员认证失败")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			response.Unauthorized(c, "Token格式错误")
			c.Abort()
			return
		}

		token := parts[1]

		if token == "" || !strings.HasPrefix(token, "admin-token-") {
			response.Forbidden(c, "无管理员权限")
			c.Abort()
			return
		}

		c.Set("is_admin", true)
		c.Set("user_id", uint(0)) // 管理员用户ID为0

		c.Next()
	}
}
