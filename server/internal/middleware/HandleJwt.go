package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"server/internal/config"
	"server/internal/repository"
	"server/internal/model/dto"
	"server/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/sync/singleflight"
)

var sfGroup singleflight.Group

// AuthToken 用户登录Token拦截
func AuthToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头中获取token
		authToken := c.Request.Header.Get("auth-token")
		// 如果没有登录信息则直接清退
		if authToken == "" {
			dto.CustomResult(http.StatusUnauthorized, dto.SUCCESS, nil, "用户未授权,请先登录", c)
			c.Abort()
			return
		}
		// 解析token中的信息
		uc, err := utils.ParseToken(authToken)
		if err != nil && !errors.Is(err, jwt.ErrTokenExpired) {
			dto.CustomResult(http.StatusUnauthorized, dto.SUCCESS, nil, "无效的口令,请重新登录!!!", c)
			c.Abort()
			return
		}
		if uc == nil {
			dto.CustomResult(http.StatusUnauthorized, dto.SUCCESS, nil, "无效的口令,请重新登录!!!", c)
			c.Abort()
			return
		}

		// 处理 token 刷新逻辑（针对过期但 Redis 中有效的 token）
		if err != nil && errors.Is(err, jwt.ErrTokenExpired) {
			// 使用 singleflight 防止并发刷新
			key := fmt.Sprintf("refresh:%d", uc.UserID)
			val, _, _ := sfGroup.Do(key, func() (any, error) {
				// 再次检查 Redis 中的 token
				t := repository.GetUserTokenById(uc.UserID)
				if len(t) <= 0 || t != authToken {
					return nil, errors.New("invalid or expired token in redis")
				}
				// 生成新token
				newToken, _ := utils.GenToken(uc.UserID, uc.UserName)
				_ = repository.SaveUserToken(newToken, uc.UserID)
				return newToken, nil
			})

			if val == nil {
				dto.CustomResult(http.StatusUnauthorized, dto.SUCCESS, nil, "身份验证信息已失效,请重新登录!!!", c)
				c.Abort()
				return
			}
			newToken := val.(string)
			// 解析出新的 UserClaims
			uc, _ = utils.ParseToken(newToken)
			c.Header("new-token", newToken)
		} else {
			// 未过期的情况，仅校验 Redis 一致性
			t := repository.GetUserTokenById(uc.UserID)
			if len(t) <= 0 {
				dto.CustomResult(http.StatusUnauthorized, dto.SUCCESS, nil, "身份验证信息已失效,请重新登录!!!", c)
				c.Abort()
				return
			}
			if t != authToken {
				dto.CustomResult(http.StatusUnauthorized, dto.SUCCESS, nil, "账号在其它设备登录,身份验证信息失效,请重新登录!!!", c)
				c.Abort()
				return
			}
		}

		// 将UserClaims存放到context中
		c.Set(config.AuthUserClaims, uc)
		c.Next()
	}
}
