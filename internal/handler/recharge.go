package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"vrec/internal/middleware"
	"vrec/internal/model"
	"vrec/internal/service"
	"vrec/pkg/errors"
	"vrec/pkg/response"
)

type RechargeHandler struct {
	rechargeService *service.RechargeService
}

func NewRechargeHandler(rechargeService *service.RechargeService) *RechargeHandler {
	return &RechargeHandler{rechargeService: rechargeService}
}

type CreateRechargeReq struct {
	Amount     string `json:"amount" binding:"required"`
	PayChannel string `json:"pay_channel" binding:"required,oneof=alipay wechat"`
}

func (h *RechargeHandler) CreateRecharge(c *gin.Context) {
	userID := middleware.GetUserID(c)
	tokenID := middleware.GetTokenID(c)

	var req CreateRechargeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters: "+err.Error())
		return
	}

	order, err := h.rechargeService.CreatePayOrder(c.Request.Context(), userID, tokenID, req.Amount, model.PayChannel(req.PayChannel))
	if err != nil {
		if errors.Is(err, errors.ErrPayChannelInvalid) {
			response.Error(c, errors.ErrPayChannelInvalid.Code, errors.ErrPayChannelInvalid.Msg)
			return
		}
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	response.Success(c, gin.H{
		"recharge_no": order.RechargeNo.String(),
		"pay_url":      order.PayURL,
		"amount":       order.Amount.String(),
		"pay_channel": order.PayChannel,
		"expires_at":   order.ExpiresAt,
	})
}

type GetRechargeReq struct {
	RechargeNo string `uri:"recharge_no" binding:"required"`
}

func (h *RechargeHandler) GetRecharge(c *gin.Context) {
	var req GetRechargeReq
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters")
		return
	}

	rechargeNo, err := uuid.Parse(req.RechargeNo)
	if err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid recharge_no format")
		return
	}

	order, err := h.rechargeService.GetByRechargeNo(c.Request.Context(), rechargeNo)
	if err != nil {
		if errors.Is(err, errors.ErrRechargeNotFound) {
			response.Error(c, errors.ErrRechargeNotFound.Code, errors.ErrRechargeNotFound.Msg)
			return
		}
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	response.Success(c, gin.H{
		"recharge_no": order.RechargeNo.String(),
		"status":     order.Status,
		"amount":     order.Amount.String(),
		"pay_channel": order.PayChannel,
		"pay_url":    order.PayURL,
		"paid_at":    order.PaidAt,
		"expires_at": order.ExpiresAt,
		"created_at": order.CreatedAt,
	})
}

type ListRechargesReq struct {
	Limit        string `form:"limit" binding:"omitempty,min=1,max=100"`
	AfterOrderNo string `form:"after_order_no" binding:"omitempty"`
	AfterTime    string `form:"after_time" binding:"omitempty"`
}

func (h *RechargeHandler) ListRechargeOrders(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req ListRechargesReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters")
		return
	}

	limit := 20
	if req.Limit != "" {
		limit, _ = strconv.Atoi(req.Limit)
	}
	afterOrderNo := req.AfterOrderNo
	var afterCreatedAt time.Time
	if req.AfterTime != "" {
		afterCreatedAt, _ = time.Parse(time.RFC3339, req.AfterTime)
	}

	orders, err := h.rechargeService.GetUserRechargeOrders(c.Request.Context(), userID, limit, afterOrderNo, afterCreatedAt)
	if err != nil {
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	response.Success(c, gin.H{
		"orders": orders,
	})
}
