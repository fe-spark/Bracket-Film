package middleware

import (
	"errors"
	"net/http"
	"server-v2/config"
	"server-v2/internal/repository"
	"server-v2/pkg/response"
	"server-v2/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthToken 用户登录Token拦截
func AuthToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头中获取token
		authToken := c.Request.Header.Get("auth-token")
		// 如果没有登录信息则直接清退
		if authToken == "" {
			response.CustomResult(http.StatusUnauthorized, response.SUCCESS, nil, "用户未授权,请先登录", c)
			c.Abort()
			return
		}
		// 解析token中的信息
		uc, err := utils.ParseToken(authToken)
		if err != nil && !errors.Is(err, jwt.ErrTokenExpired) {
			response.CustomResult(http.StatusUnauthorized, response.SUCCESS, nil, "无效的口令,请重新登录!!!", c)
			c.Abort()
			return
		}
		if uc == nil {
			response.CustomResult(http.StatusUnauthorized, response.SUCCESS, nil, "无效的口令,请重新登录!!!", c)
			c.Abort()
			return
		}
		// 从Redis中获取对应的token是否存在, 如果存在则刷新token
		t := repository.GetUserTokenById(uc.UserID)
		// 如果 redis中获取的token为空则登录已过期需重新登录
		if len(t) <= 0 {
			response.CustomResult(http.StatusUnauthorized, response.SUCCESS, nil, "身份验证信息已失效,请重新登录!!!", c)
			c.Abort()
			return
		}
		// 如果redis中存在对应token, 校验authToken是否与redis中的一致
		if t != authToken {
			// 如果不一致则证明authToken已经失效或在其他地方登录, 则需要重新登录
			response.CustomResult(http.StatusUnauthorized, response.SUCCESS, nil, "账号在其它设备登录,身份验证信息失效,请重新登录!!!", c)
			c.Abort()
			return
		} else if err != nil && errors.Is(err, jwt.ErrTokenExpired) {
			// 如果token已经过期,且redis中的token与authToken 相同则更新 token
			// 生成新token
			newToken, _ := utils.GenToken(uc.UserID, uc.UserName)
			// 将新token同步到redis中
			_ = repository.SaveUserToken(newToken, uc.UserID)
			// 解析出新的 UserClaims
			uc, _ = utils.ParseToken(newToken)
			c.Header("new-token", newToken)
		}
		// 将UserClaims存放到context中
		c.Set(config.AuthUserClaims, uc)
		c.Next()
	}
}
