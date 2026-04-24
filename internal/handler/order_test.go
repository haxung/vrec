package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCreateOrderReq_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "missing audio_url",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty audio_url",
			body:       `{"audio_url":""}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")

			var req CreateOrderReq
			err := c.ShouldBindJSON(&req)

			if tt.wantStatus == http.StatusBadRequest {
				assert.Error(t, err)
			}
		})
	}
}

func TestGetOrderReq_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		url        string
		wantStatus int
	}{
		{
			name:       "missing order_no",
			url:        "/orders/",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid order_no format",
			url:        "/orders/not-a-uuid",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, tt.url, nil)

			var req GetOrderReq
			err := c.ShouldBindUri(&req)

			if tt.wantStatus == http.StatusBadRequest {
				assert.Error(t, err)
			}
		})
	}
}
