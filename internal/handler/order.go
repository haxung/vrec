package handler

import (
	"encoding/json"
	"mime"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"vrec/internal/middleware"
	"vrec/internal/model"
	"vrec/internal/service"
	"vrec/pkg/errors"
	"vrec/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type OrderHandler struct {
	orderService    *service.OrderService
	rechargeService *service.RechargeService
	resultService   *service.TranscriptionResultService
	audioService    *service.AudioService
	s3Service       *service.S3Service
	asrService      *service.ASRService
	subtitleService *service.SubtitleService
	meetingService  *service.MeetingNoteService
}

func NewOrderHandler(orderService *service.OrderService, rechargeService *service.RechargeService, resultService *service.TranscriptionResultService, audioService *service.AudioService, s3Service *service.S3Service, asrService *service.ASRService, subtitleService *service.SubtitleService, meetingService *service.MeetingNoteService) *OrderHandler {
	return &OrderHandler{
		orderService:    orderService,
		rechargeService: rechargeService,
		resultService:   resultService,
		audioService:    audioService,
		s3Service:       s3Service,
		asrService:      asrService,
		subtitleService: subtitleService,
		meetingService:  meetingService,
	}
}

type CreateOrderReq struct {
	AudioURL        string `json:"audio_url" binding:"required"`
	NeedSubtitle    bool   `json:"need_subtitle"`
	NeedMeetingNote bool   `json:"need_meeting_note"`
	CallbackURL     string `json:"callback_url"`
}

// 流式上传（multipart form）
func (h *OrderHandler) CreateOrderStream(c *gin.Context) {
	userID := middleware.GetUserID(c)
	tokenID := middleware.GetTokenID(c)

	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "missing audio file")
		return
	}
	defer file.Close()

	audioInfo, err := h.audioService.ParseFromReader(c.Request.Context(), file, header.Filename)
	if err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "failed to parse audio")
		return
	}

	if err := h.audioService.ValidateAudio(audioInfo); err != nil {
		if strings.Contains(err.Error(), "size") {
			response.Error(c, errors.ErrAudioTooLarge.Code, errors.ErrAudioTooLarge.Msg)
			return
		}
		if strings.Contains(err.Error(), "duration") {
			response.Error(c, errors.ErrAudioTooLong.Code, errors.ErrAudioTooLong.Msg)
			return
		}
		response.Error(c, errors.ErrInvalidParams.Code, err.Error())
		return
	}

	// 上传到 S3
	s3Key := h.s3Service.GenerateKeyWithFilename("audio", header.Filename)
	contentType := mime.TypeByExtension(filepath.Ext(header.Filename))
	uploadResult, err := h.s3Service.Upload(c.Request.Context(), s3Key, file, audioInfo.Size, contentType)
	if err != nil {
		response.Error(c, errors.ErrInternalError.Code, "failed to upload to s3")
		return
	}

	a, _ := strconv.ParseBool(c.Request.FormValue("need_subtitle"))
	b, _ := strconv.ParseBool(c.Request.FormValue("need_meeting_note"))
	callbackURL := c.Request.FormValue("callback_url")
	order, err := h.orderService.CreateOrder(c.Request.Context(), userID, tokenID, uploadResult.URL, model.OrderSourceStream, audioInfo, audioInfo.Size, a, b, callbackURL)
	if err != nil {
		if errors.Is(err, errors.ErrInsufficientBalance) {
			response.Error(c, errors.ErrInsufficientBalance.Code, errors.ErrInsufficientBalance.Msg)
			return
		}
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	// 更新 S3 信息
	h.orderService.UpdateS3Info(c.Request.Context(), order.OrderNo, uploadResult.URL, s3Key, &uploadResult.ExpiresAt)

	// 提交 ASR 转写
	_, err = h.asrService.Transcribe(c.Request.Context(), &service.TranscribeRequest{
		AudioURL:    uploadResult.URL,
		CallbackURL: h.asrService.GetCallbackURL(),
		OrderNo:     order.OrderNo,
	})
	if err != nil {
		h.orderService.UpdateStatus(c.Request.Context(), order.OrderNo, model.OrderStatusFailed)
		response.Error(c, errors.ErrInternalError.Code, "failed to submit asr task")
		return
	}
	h.orderService.UpdateStatus(c.Request.Context(), order.OrderNo, model.OrderStatusProcessing)

	response.Success(c, gin.H{
		"order_no":       order.OrderNo.String(),
		"status":         model.OrderStatusProcessing,
		"audio_duration": order.AudioDuration,
		"storage_cost":   order.StorageCost.String(),
		"asr_cost":       order.ASRCost.String(),
		"total_cost":     order.TotalCost.String(),
	})
}

// 远程 URL
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	userID := middleware.GetUserID(c)
	tokenID := middleware.GetTokenID(c)

	var req CreateOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters: "+err.Error())
		return
	}

	audioInfo, reader, err := h.audioService.ParseFromURL(c.Request.Context(), req.AudioURL)
	if err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "failed to parse audio from url")
		return
	}

	if audioInfo == nil {
		response.Error(c, errors.ErrInvalidParams.Code, "failed to get audio info")
		return
	}

	if err := h.audioService.ValidateAudio(audioInfo); err != nil {
		if strings.Contains(err.Error(), "size") {
			response.Error(c, errors.ErrAudioTooLarge.Code, errors.ErrAudioTooLarge.Msg)
			return
		}
		if strings.Contains(err.Error(), "duration") {
			response.Error(c, errors.ErrAudioTooLong.Code, errors.ErrAudioTooLong.Msg)
			return
		}
		response.Error(c, errors.ErrInvalidParams.Code, err.Error())
		return
	}

	// 下载后上传到 S3
	// 从 URL 中提取文件名
	filename := extractFilenameFromURL(req.AudioURL)
	s3Key := h.s3Service.GenerateKeyWithFilename("audio", filename)
	contentType := mime.TypeByExtension(filepath.Ext(filename))
	uploadResult, err := h.s3Service.Upload(c.Request.Context(), s3Key, reader, audioInfo.Size, contentType)
	if err != nil {
		response.Error(c, errors.ErrInternalError.Code, "failed to upload to s3")
		return
	}

	order, err := h.orderService.CreateOrder(c.Request.Context(), userID, tokenID, req.AudioURL, model.OrderSourceRemote, audioInfo, audioInfo.Size, req.NeedSubtitle, req.NeedMeetingNote, req.CallbackURL)
	if err != nil {
		if errors.Is(err, errors.ErrInsufficientBalance) {
			response.Error(c, errors.ErrInsufficientBalance.Code, errors.ErrInsufficientBalance.Msg)
			return
		}
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	// 更新 S3 信息
	h.orderService.UpdateS3Info(c.Request.Context(), order.OrderNo, uploadResult.URL, s3Key, &uploadResult.ExpiresAt)

	// 提交 ASR 转写
	_, err = h.asrService.Transcribe(c.Request.Context(), &service.TranscribeRequest{
		AudioURL:    uploadResult.URL,
		CallbackURL: h.asrService.GetCallbackURL(),
		OrderNo:     order.OrderNo,
	})
	if err != nil {
		h.orderService.UpdateStatus(c.Request.Context(), order.OrderNo, model.OrderStatusFailed)
		response.Error(c, errors.ErrInternalError.Code, "failed to submit asr task")
		return
	}
	h.orderService.UpdateStatus(c.Request.Context(), order.OrderNo, model.OrderStatusProcessing)

	response.Success(c, gin.H{
		"order_no":       order.OrderNo.String(),
		"status":         model.OrderStatusProcessing,
		"audio_duration": order.AudioDuration,
		"storage_cost":   order.StorageCost.String(),
		"asr_cost":       order.ASRCost.String(),
		"total_cost":     order.TotalCost.String(),
	})
}

type GetOrderReq struct {
	OrderNo string `uri:"order_no" binding:"required"`
}

func (h *OrderHandler) GetOrder(c *gin.Context) {
	var req GetOrderReq
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters: "+err.Error())
		return
	}

	orderNo, err := uuid.Parse(req.OrderNo)
	if err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid order_no format")
		return
	}

	order, err := h.orderService.GetByOrderNo(c.Request.Context(), orderNo)
	if err != nil {
		if errors.Is(err, errors.ErrOrderNotFound) {
			response.Error(c, errors.ErrOrderNotFound.Code, errors.ErrOrderNotFound.Msg)
			return
		}
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	s3Expired := h.orderService.IsS3URLExpired(order)
	response.Success(c, gin.H{
		"order_no":       order.OrderNo.String(),
		"status":         order.Status,
		"original_url":   order.OriginalURL,
		"audio_duration": order.AudioDuration,
		"audio_format":   order.AudioFormat,
		"sample_rate":    order.SampleRate,
		"channels":       order.Channels,
		"bit_rate":       order.BitRate,
		"codec":          order.Codec,
		"s3_url":         order.S3URL,
		"s3_expired":     s3Expired,
		"storage_cost":   order.StorageCost.String(),
		"asr_cost":       order.ASRCost.String(),
		"total_cost":     order.TotalCost.String(),
		"created_at":     order.CreatedAt,
	})
}

type ListOrdersReq struct {
	Limit        string `form:"limit" binding:"omitempty,min=1,max=100"`
	AfterOrderNo string `form:"after_order_no" binding:"omitempty"`
	AfterTime    string `form:"after_time" binding:"omitempty"` // RFC3339 format
}

func (h *OrderHandler) ListOrders(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req ListOrdersReq
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

	orders, err := h.orderService.GetUserOrders(c.Request.Context(), userID, limit, afterOrderNo, afterCreatedAt)
	if err != nil {
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	response.Success(c, gin.H{
		"orders": orders,
	})
}

type GetResultReq struct {
	OrderNo string `uri:"order_no" binding:"required"`
}

func (h *OrderHandler) GetResult(c *gin.Context) {
	var req GetResultReq
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters")
		return
	}

	orderNo, err := uuid.Parse(req.OrderNo)
	if err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid order_no format")
		return
	}

	result, err := h.resultService.GetByOrderNo(c.Request.Context(), orderNo)
	if err != nil {
		if errors.Is(err, errors.ErrResultNotFound) {
			response.Error(c, errors.ErrResultNotFound.Code, errors.ErrResultNotFound.Msg)
			return
		}
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	// 获取实际内容：优先使用 ResultText，否则从 S3 下载
	resultText := result.ResultText
	if resultText == "" && result.ResultS3Key != "" {
		data, err := h.s3Service.Download(c.Request.Context(), result.ResultS3Key)
		if err == nil && len(data) > 0 {
			resultText = string(data)
		}
	}

	// 解析 ASR 原始结果
	var asrResult model.TranscriptionResponse
	if err := json.Unmarshal([]byte(resultText), &asrResult); err != nil {
		// 如果解析失败，直接返回原始文本
		response.Success(c, gin.H{
			"order_no":    result.OrderNo.String(),
			"result_text": resultText,
		})
		return
	}

	// 返回与 ASR 格式一致的结果
	response.Success(c, gin.H{
		"order_no":    result.OrderNo.String(),
		"file_url":    asrResult.FileURL,
		"properties":  asrResult.Properties,
		"transcripts": asrResult.Transcripts,
	})
}

type CancelOrderReq struct {
	OrderNo string `uri:"order_no" binding:"required"`
}

func (h *OrderHandler) CancelOrder(c *gin.Context) {
	var req CancelOrderReq
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters")
		return
	}

	orderNo, err := uuid.Parse(req.OrderNo)
	if err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid order_no format")
		return
	}

	// 获取订单以获取 task_id
	order, err := h.orderService.GetByOrderNo(c.Request.Context(), orderNo)
	if err != nil {
		if errors.Is(err, errors.ErrOrderNotFound) {
			response.Error(c, errors.ErrOrderNotFound.Code, errors.ErrOrderNotFound.Msg)
			return
		}
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	if err := h.orderService.CancelOrder(c.Request.Context(), orderNo, order.TaskID); err != nil {
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	response.Success(c, nil)
}

// 生成字幕
type GenerateSubtitleReq struct {
	OrderNo string `uri:"order_no" binding:"required"`
}

func (h *OrderHandler) GenerateSubtitle(c *gin.Context) {
	var req GenerateSubtitleReq
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters")
		return
	}

	orderNo, err := uuid.Parse(req.OrderNo)
	if err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid order_no format")
		return
	}

	// 获取订单确认状态
	order, err := h.orderService.GetByOrderNo(c.Request.Context(), orderNo)
	if err != nil {
		if errors.Is(err, errors.ErrOrderNotFound) {
			response.Error(c, errors.ErrOrderNotFound.Code, errors.ErrOrderNotFound.Msg)
			return
		}
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	if order.Status != model.OrderStatusSuccess {
		response.Error(c, errors.ErrOrderStatusInvalid.Code, "order not succeeded yet")
		return
	}

	result, err := h.subtitleService.GenerateSubtitle(c.Request.Context(), &service.GenerateSubtitleRequest{
		OrderNo:  orderNo,
		AudioURL: order.S3URL,
	})
	if err != nil {
		response.Error(c, errors.ErrInternalError.Code, err.Error())
		return
	}

	subtitleContent, _ := h.subtitleService.GetSubtitleContent(c.Request.Context(), orderNo)
	response.Success(c, gin.H{
		"order_no":         result.OrderNo.String(),
		"subtitle_content": subtitleContent,
		"cost":             h.subtitleService.GetSubtitleCost(float64(order.AudioDuration / 60)),
	})
}

// 获取字幕
type GetSubtitleReq struct {
	OrderNo string `uri:"order_no" binding:"required"`
}

func (h *OrderHandler) GetSubtitle(c *gin.Context) {
	var req GetSubtitleReq
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters")
		return
	}

	orderNo, err := uuid.Parse(req.OrderNo)
	if err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid order_no format")
		return
	}

	subtitleContent, err := h.subtitleService.GetSubtitleContent(c.Request.Context(), orderNo)
	if err != nil {
		if errors.Is(err, errors.ErrResultNotFound) {
			response.Error(c, errors.ErrSubtitleNotFound.Code, errors.ErrSubtitleNotFound.Msg)
			return
		}
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	response.Success(c, gin.H{
		"order_no":         orderNo.String(),
		"subtitle_content": subtitleContent,
	})
}

// 生成会议纪要
type GenerateMeetingNoteReq struct {
	OrderNo string `uri:"order_no" binding:"required"`
}

func (h *OrderHandler) GenerateMeetingNote(c *gin.Context) {
	var req GenerateMeetingNoteReq
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters")
		return
	}

	orderNo, err := uuid.Parse(req.OrderNo)
	if err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid order_no format")
		return
	}

	// 获取订单确认状态
	order, err := h.orderService.GetByOrderNo(c.Request.Context(), orderNo)
	if err != nil {
		if errors.Is(err, errors.ErrOrderNotFound) {
			response.Error(c, errors.ErrOrderNotFound.Code, errors.ErrOrderNotFound.Msg)
			return
		}
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	if order.Status != model.OrderStatusSuccess {
		response.Error(c, errors.ErrOrderStatusInvalid.Code, "order not succeeded yet")
		return
	}

	summary, err := h.meetingService.GenerateMeetingNote(c.Request.Context(), &service.GenerateMeetingNoteRequest{
		OrderNo: orderNo,
	})
	if err != nil {
		if !h.meetingService.IsLLMEnabled() {
			response.Error(c, errors.ErrLLMNotEnabled.Code, errors.ErrLLMNotEnabled.Msg)
			return
		}
		response.Error(c, errors.ErrInternalError.Code, err.Error())
		return
	}

	response.Success(c, gin.H{
		"order_no":     summary.OrderNo.String(),
		"summary_text": summary.SummaryText,
		"cost":         summary.Cost.String(),
	})
}

// 获取会议纪要
type GetMeetingNoteReq struct {
	OrderNo string `uri:"order_no" binding:"required"`
}

func (h *OrderHandler) GetMeetingNote(c *gin.Context) {
	var req GetMeetingNoteReq
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters")
		return
	}

	orderNo, err := uuid.Parse(req.OrderNo)
	if err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid order_no format")
		return
	}

	summaryText, err := h.meetingService.GetMeetingNoteContent(c.Request.Context(), orderNo)
	if err != nil {
		response.Error(c, errors.ErrMeetingNoteNotFound.Code, errors.ErrMeetingNoteNotFound.Msg)
		return
	}

	response.Success(c, gin.H{
		"order_no":     orderNo.String(),
		"summary_text": summaryText,
	})
}

// 查询余额不足的订单列表
type ListInsufficientOrdersReq struct {
	Limit        string `form:"limit"`
	AfterOrderNo string `form:"after_order_no"`
	AfterTime    string `form:"after_time"`
}

func (h *OrderHandler) ListInsufficientOrders(c *gin.Context) {
	userID := middleware.GetUserID(c)

	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = min(parsed, 100)
		}
	}
	afterOrderNo := c.Query("after_order_no")
	var afterCreatedAt time.Time
	if a := c.Query("after_time"); a != "" {
		afterCreatedAt, _ = time.Parse(time.RFC3339, a)
	}

	orders, err := h.orderService.GetInsufficientOrders(c.Request.Context(), userID, limit, afterOrderNo, afterCreatedAt)
	if err != nil {
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	response.Success(c, gin.H{
		"orders": orders,
	})
}

// 批量重试余额不足的订单
type RetryOrdersReq struct {
	OrderNos []string `json:"order_nos" binding:"required"`
}

func (h *OrderHandler) RetryInsufficientOrder(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req RetryOrdersReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters")
		return
	}

	var success []string
	var failed []map[string]string

	for _, orderNoStr := range req.OrderNos {
		orderNo, err := uuid.Parse(orderNoStr)
		if err != nil {
			failed = append(failed, map[string]string{
				"order_no": orderNoStr,
				"error":    "invalid order_no format",
			})
			continue
		}

		// 验证订单属于当前用户
		order, err := h.orderService.GetByOrderNo(c.Request.Context(), orderNo)
		if err != nil || order == nil {
			failed = append(failed, map[string]string{
				"order_no": orderNoStr,
				"error":    "order not found",
			})
			continue
		}
		if order.UserID != userID {
			failed = append(failed, map[string]string{
				"order_no": orderNoStr,
				"error":    "order not found",
			})
			continue
		}

		// 重试订单
		if err := h.orderService.RetryInsufficientOrder(c.Request.Context(), orderNo); err != nil {
			failed = append(failed, map[string]string{
				"order_no": orderNoStr,
				"error":    err.Error(),
			})
			continue
		}
		success = append(success, orderNoStr)
	}

	response.Success(c, gin.H{
		"success": success,
		"failed":  failed,
	})
}

// 查询订单费用
type GetOrderCostReq struct {
	OrderNo string `uri:"order_no" binding:"required"`
}

func (h *OrderHandler) GetOrderCost(c *gin.Context) {
	var req GetOrderCostReq
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters")
		return
	}

	orderNo, err := uuid.Parse(req.OrderNo)
	if err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid order_no format")
		return
	}

	order, err := h.orderService.GetByOrderNo(c.Request.Context(), orderNo)
	if err != nil || order == nil {
		response.Error(c, errors.ErrOrderNotFound.Code, errors.ErrOrderNotFound.Msg)
		return
	}

	response.Success(c, gin.H{
		"order_no":      order.OrderNo.String(),
		"storage_cost":  order.StorageCost.String(),
		"asr_cost":      order.ASRCost.String(),
		"subtitle_cost": order.SubtitleCost.String(),
		"meeting_cost":  order.MeetingCost.String(),
		"total_cost":    order.TotalCost.String(),
	})
}

// 查询用户账单
type GetBillsReq struct {
	StartTime string `form:"start_time" binding:"required"` // RFC3339 format
	EndTime   string `form:"end_time" binding:"required"`   // RFC3339 format
	Page      string `form:"page" binding:"omitempty,min=1"`
	PageSize  string `form:"page_size" binding:"omitempty,min=1,max=100"`
}

func (h *OrderHandler) GetBills(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req GetBillsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid parameters")
		return
	}

	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid start_time format, use RFC3339")
		return
	}
	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		response.Error(c, errors.ErrInvalidParams.Code, "invalid end_time format, use RFC3339")
		return
	}

	// 校验时间跨度不超过1年
	if endTime.Sub(startTime) > 365*24*time.Hour {
		response.Error(c, errors.ErrInvalidParams.Code, "time range cannot exceed 1 year")
		return
	}

	// 分页参数
	page := 1
	pageSize := 20
	if req.Page != "" {
		page, _ = strconv.Atoi(req.Page)
	}
	if req.PageSize != "" {
		pageSize, _ = strconv.Atoi(req.PageSize)
		if pageSize > 100 {
			pageSize = 100
		}
	}

	// 查询订单费用（不返回大字段）
	orders, err := h.orderService.GetOrdersForBill(c.Request.Context(), userID, startTime, endTime)
	if err != nil {
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	// 查询充值记录（不返回大字段）
	recharges, err := h.rechargeService.GetRechargeOrdersForBill(c.Request.Context(), userID, startTime, endTime)
	if err != nil {
		response.Error(c, errors.ErrInternalError.Code, errors.ErrInternalError.Msg)
		return
	}

	// 按月分组计算账单
	monthlyBills := make(map[string]*MonthlyBill)
	for _, order := range orders {
		monthKey := order.CreatedAt.Format("2006-01")
		if _, ok := monthlyBills[monthKey]; !ok {
			monthlyBills[monthKey] = &MonthlyBill{
				Month:         monthKey,
				TotalCost:     decimal.Zero,
				TotalRecharge: decimal.Zero,
				OrderCount:    0,
				RechargeCount: 0,
			}
		}
		monthlyBills[monthKey].Orders = append(monthlyBills[monthKey].Orders, OrderCost{
			OrderNo:      order.OrderNo.String(),
			StorageCost:  order.StorageCost.String(),
			ASRCost:      order.ASRCost.String(),
			SubtitleCost: order.SubtitleCost.String(),
			MeetingCost:  order.MeetingCost.String(),
			TotalCost:    order.TotalCost.String(),
			CreatedAt:    order.CreatedAt,
		})
		monthlyBills[monthKey].TotalCost = monthlyBills[monthKey].TotalCost.Add(order.TotalCost)
		monthlyBills[monthKey].OrderCount++
	}

	for _, recharge := range recharges {
		monthKey := recharge.CreatedAt.Format("2006-01")
		if _, ok := monthlyBills[monthKey]; !ok {
			monthlyBills[monthKey] = &MonthlyBill{
				Month:         monthKey,
				TotalCost:     decimal.Zero,
				TotalRecharge: decimal.Zero,
			}
		}
		monthlyBills[monthKey].Recharges = append(monthlyBills[monthKey].Recharges, RechargeRecord{
			RechargeNo: recharge.RechargeNo.String(),
			Amount:     recharge.Amount.String(),
			CreatedAt:  recharge.CreatedAt,
		})
		monthlyBills[monthKey].TotalRecharge = monthlyBills[monthKey].TotalRecharge.Add(recharge.Amount)
		monthlyBills[monthKey].RechargeCount++
	}

	// 转换为数组并排序
	var allBills []*MonthlyBill
	for _, bill := range monthlyBills {
		allBills = append(allBills, bill)
	}
	sortMonthlyBills(allBills)

	// 分页
	total := len(allBills)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= total {
		start = 0
		end = 0
	} else if end > total {
		end = total
	}
	pagedBills := allBills[start:end]

	response.Success(c, gin.H{
		"start_time": startTime.Format(time.RFC3339),
		"end_time":   endTime.Format(time.RFC3339),
		"total":      total,
		"page":       page,
		"page_size":  pageSize,
		"bills":      pagedBills,
	})
}

type OrderCost struct {
	OrderNo      string    `json:"order_no"`
	StorageCost  string    `json:"storage_cost"`
	ASRCost      string    `json:"asr_cost"`
	SubtitleCost string    `json:"subtitle_cost"`
	MeetingCost  string    `json:"meeting_cost"`
	TotalCost    string    `json:"total_cost"`
	CreatedAt    time.Time `json:"created_at"`
}

type RechargeRecord struct {
	RechargeNo string    `json:"recharge_no"`
	Amount     string    `json:"amount"`
	CreatedAt  time.Time `json:"created_at"`
}

type MonthlyBill struct {
	Month         string           `json:"month"`
	Orders        []OrderCost      `json:"orders,omitempty"`
	Recharges     []RechargeRecord `json:"recharges,omitempty"`
	TotalCost     decimal.Decimal  `json:"total_cost"`
	TotalRecharge decimal.Decimal  `json:"total_recharge"`
	OrderCount    int              `json:"order_count"`
	RechargeCount int              `json:"recharge_count"`
}

func sortMonthlyBills(bills []*MonthlyBill) {
	sort.Slice(bills, func(i, j int) bool {
		return bills[i].Month < bills[j].Month
	})
}

// extractFilenameFromURL 从 URL 中提取文件名
func extractFilenameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "audio.mp3" // 默认文件名
	}
	filename := filepath.Base(u.Path)
	if filename == "" || filename == "." || filename == "/" {
		return "audio.mp3"
	}
	return filename
}
