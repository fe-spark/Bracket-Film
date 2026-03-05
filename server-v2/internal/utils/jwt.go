package utils

import (
	"errors"
	"server-v2/internal/config"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type UserClaims struct {
	UserID   uint   `json:"userID"`
	UserName string `json:"userName"`
	jwt.RegisteredClaims
}

// GenToken 生成token
func GenToken(userId uint, userName string) (string, error) {
	uc := UserClaims{
		UserID:   userId,
		UserName: userName,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    config.Issuer,
			Subject:   userName,
			Audience:  jwt.ClaimStrings{"Auth_All"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(config.AuthTokenExpires * time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-10 * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        GenerateSalt(),
		},
	}
	priKey, err := ParsePriKeyBytes([]byte(config.PrivateKey))
	if err != nil {
		return "", err
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, uc).SignedString(priKey)
	return token, err
}

// ParseToken 解析token
func ParseToken(tokenStr string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		pub, err := ParsePubKeyBytes([]byte(config.PublicKey))
		if err != nil {
			return nil, err
		}
		return pub, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			claims, _ := token.Claims.(*UserClaims)
			return claims, err
		}
		return nil, err
	}
	// 验证token是否有效
	if !token.Valid {
		return nil, errors.New("token is invalid")
	}
	// 解析token中的claims内容
	claims, ok := token.Claims.(*UserClaims)
	if !ok {
		return nil, errors.New("invalid claim type error")
	}
	return claims, err
}
