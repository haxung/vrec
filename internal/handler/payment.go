package handler

import (
	"github.com/gin-gonic/gin"
	"vrec/internal/service"
	"vrec/pkg/errors"
	"vrec/pkg/response"
)

type PaymentCallbackHandler struct {
	rechargeService *service.RechargeService
}

func NewPaymentCallbackHandler(rechargeService *service.RechargeService) *PaymentCallbackHandler {
	return &PaymentCallbackHandler{rechargeService: rechargeService}
}

type AlipayCallbackReq struct {
	TradeNo   string `json:"trade_no"`
	Status    string `json:"status"`
	Amount    string `json:"amount"`
	TradeNote string `json:"trade_note"`
}

func (h *PaymentCallbackHandler) AlipayCallback(c *gin.Context) {
	var req AlipayCallbackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters")
		return
	}

	paid := req.Status == "TRADE_SUCCESS"
	if err := h.rechargeService.HandleCallback(c.Request.Context(), req.TradeNo, paid); err != nil {
		response.Error(c, errors.ErrInternalError.Code, "callback处理失败")
		return
	}

	response.Success(c, nil)
}

type WechatCallbackReq struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	Amount        string `json:"amount"`
}

func (h *PaymentCallbackHandler) WechatCallback(c *gin.Context) {
	var req WechatCallbackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters")
		return
	}

	paid := req.Status == "SUCCESS"
	if err := h.rechargeService.HandleCallback(c.Request.Context(), req.TransactionID, paid); err != nil {
		response.Error(c, errors.ErrInternalError.Code, "callback处理失败")
		return
	}

	response.Success(c, nil)
}
