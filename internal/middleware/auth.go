package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"vrec/internal/config"
	"vrec/internal/service"
	"vrec/pkg/errors"
	"vrec/pkg/response"
)

const (
	AuthUserIDKey  = "user_id"
	AuthTokenIDKey = "token_id"
	BearerPrefix   = "Bearer "
)

type AuthMiddleware struct {
	userService *service.UserService
	cfg        *config.AuthConfig
}

func NewAuthMiddleware(userService *service.UserService, cfg *config.AuthConfig) *AuthMiddleware {
	return &AuthMiddleware{userService: userService, cfg: cfg}
}

type JWTClaims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

func (m *AuthMiddleware) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.ErrorWithStatus(c, http.StatusUnauthorized, errors.ErrAuthMissingHeader.Code, errors.ErrAuthMissingHeader.Msg)
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, BearerPrefix) {
			response.ErrorWithStatus(c, http.StatusUnauthorized, errors.ErrAuthMissingHeader.Code, errors.ErrAuthMissingHeader.Msg)
			c.Abort()
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, BearerPrefix)

		// JWT 模式
		if m.cfg.JWTEnabled {
			userID, tokenID, err := m.parseJWT(tokenStr)
			if err != nil {
				response.ErrorWithStatus(c, http.StatusUnauthorized, errors.ErrAuthInvalidToken.Code, errors.ErrAuthInvalidToken.Msg)
				c.Abort()
				return
			}
			c.Set(AuthUserIDKey, userID)
			c.Set(AuthTokenIDKey, tokenID)
			c.Next()
			return
		}

		// UUID Token 模式（默认）
		userID, tokenID, err := m.userService.ValidateToken(c.Request.Context(), tokenStr)
		if err != nil {
			if errors.Is(err, errors.ErrAuthTokenExpired) {
				response.ErrorWithStatus(c, http.StatusUnauthorized, errors.ErrAuthTokenExpired.Code, errors.ErrAuthTokenExpired.Msg)
			} else {
				response.ErrorWithStatus(c, http.StatusUnauthorized, errors.ErrAuthInvalidToken.Code, errors.ErrAuthInvalidToken.Msg)
			}
			c.Abort()
			return
		}

		c.Set(AuthUserIDKey, userID)
		c.Set(AuthTokenIDKey, tokenID)
		c.Next()
	}
}

func (m *AuthMiddleware) parseJWT(tokenStr string) (int64, int64, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.cfg.JWTSecret), nil
	})
	if err != nil {
		return 0, 0, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return 0, 0, errors.ErrAuthInvalidToken
	}

	// JWT 模式下 tokenID 为 0（不关联具体 token）
	return claims.UserID, 0, nil
}

// GenerateJWT 生成 JWT（供登录接口使用）
func (m *AuthMiddleware) GenerateJWT(userID int64, tokenID int64) (string, error) {
	expiresAt := time.Now().Add(time.Duration(m.cfg.TokenExpireDays) * 24 * time.Hour)

	claims := JWTClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.cfg.JWTSecret))
}

// IsJWTEnabled 检查是否启用 JWT 模式
func (m *AuthMiddleware) IsJWTEnabled() bool {
	return m.cfg != nil && m.cfg.JWTEnabled
}

// ParseTokenType 判断 token 类型
func ParseTokenType(tokenStr string) (isJWT bool, err error) {
	// 尝试解析为 JWT
	_, _, err = jwt.NewParser().ParseUnverified(tokenStr, &JWTClaims{})
	if err == nil {
		return true, nil
	}

	// 尝试解析为 UUID
	_, err = uuid.Parse(tokenStr)
	if err == nil {
		return false, nil
	}

	return false, fmt.Errorf("invalid token format")
}

func GetUserID(c *gin.Context) int64 {
	userID, exists := c.Get(AuthUserIDKey)
	if !exists {
		return 0
	}
	return userID.(int64)
}

func GetTokenID(c *gin.Context) int64 {
	tokenID, exists := c.Get(AuthTokenIDKey)
	if !exists {
		return 0
	}
	return tokenID.(int64)
}
