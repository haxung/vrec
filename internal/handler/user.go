package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"vrec/internal/middleware"
	"vrec/internal/service"
	"vrec/pkg/errors"
	"vrec/pkg/response"
)

type UserHandler struct {
	userService   *service.UserService
	authMiddleware *middleware.AuthMiddleware
}

func NewUserHandler(userService *service.UserService, authMiddleware *middleware.AuthMiddleware) *UserHandler {
	return &UserHandler{userService: userService, authMiddleware: authMiddleware}
}

type RegisterReq struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6,max=32"`
}

type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *UserHandler) Register(c *gin.Context) {
	var req RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters: "+err.Error())
		return
	}

	user, err := h.userService.Register(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, errors.ErrUserAlreadyExists) {
			response.Error(c, errors.ErrUserAlreadyExists.Code, errors.ErrUserAlreadyExists.Msg)
			return
		}
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	response.Success(c, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"balance":  user.Balance.String(),
	})
}

func (h *UserHandler) Login(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters: "+err.Error())
		return
	}

	user, token, err := h.userService.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, errors.ErrUserNotFound) || errors.Is(err, errors.ErrInvalidPassword) {
			response.Error(c, errors.ErrAuthInvalidUser.Code, "invalid username or password")
			return
		}
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	// JWT 模式下返回 JWT，否则返回 UUID Token
	if h.authMiddleware.IsJWTEnabled() {
		jwtToken, err := h.authMiddleware.GenerateJWT(user.ID, token.ID)
		if err != nil {
			response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
			return
		}
		response.Success(c, gin.H{
			"id":         user.ID,
			"username":   user.Username,
			"balance":    user.Balance.String(),
			"token":      jwtToken,
			"token_type": "jwt",
		})
		return
	}

	response.Success(c, gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"balance":    user.Balance.String(),
		"token":      token.Token.String(),
		"expires_at": token.ExpiresAt,
		"token_type": "uuid",
	})
}

func (h *UserHandler) Logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	bearerPrefix := "Bearer "
	if authHeader == "" || len(authHeader) <= len(bearerPrefix) {
		response.Error(c, errors.ErrAuthMissingHeader.Code, errors.ErrAuthMissingHeader.Msg)
		return
	}

	token := authHeader[len(bearerPrefix):]
	if err := h.userService.Logout(c.Request.Context(), token); err != nil {
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	response.Success(c, nil)
}

func (h *UserHandler) ListTokens(c *gin.Context) {
	userID := middleware.GetUserID(c)

	tokens, err := h.userService.GetUserTokens(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	result := make([]gin.H, 0, len(tokens))
	for _, t := range tokens {
		result = append(result, gin.H{
			"id":         t.ID,
			"token":      t.Token.String(),
			"created_at": t.CreatedAt,
			"expires_at": t.ExpiresAt,
		})
	}

	response.Success(c, gin.H{"tokens": result})
}

type DeleteTokenReq struct {
	TokenID string `uri:"token_id" binding:"required"`
}

func (h *UserHandler) DeleteToken(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req DeleteTokenReq
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters")
		return
	}

	tokenID, err := strconv.ParseInt(req.TokenID, 10, 64)
	if err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid token_id format")
		return
	}

	if err := h.userService.DeleteToken(c.Request.Context(), userID, tokenID); err != nil {
		if errors.Is(err, errors.ErrAuthInvalidToken) {
			response.Error(c, errors.ErrAuthInvalidToken.Code, errors.ErrAuthInvalidToken.Msg)
			return
		}
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	response.Success(c, nil)
}
