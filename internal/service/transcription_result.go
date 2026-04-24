package service

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"vrec/internal/model"
	"vrec/internal/repository"
	"vrec/pkg/errors"
)

var (
	ErrResultNotFound = errors.ErrResultNotFound
)

type TranscriptionResultService struct {
	resultRepo *repository.TranscriptionResultRepository
	logger     *zap.Logger
}

func NewTranscriptionResultService(resultRepo *repository.TranscriptionResultRepository, logger *zap.Logger) *TranscriptionResultService {
	return &TranscriptionResultService{resultRepo: resultRepo, logger: logger}
}

func (s *TranscriptionResultService) Create(ctx context.Context, orderNo uuid.UUID, resultS3Key, resultText string) (*model.TranscriptionResult, error) {
	result := &model.TranscriptionResult{
		OrderNo:     orderNo,
		ResultS3Key: resultS3Key,
		ResultText:  resultText,
	}
	if err := s.resultRepo.Create(ctx, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *TranscriptionResultService) GetByOrderNo(ctx context.Context, orderNo uuid.UUID) (*model.TranscriptionResult, error) {
	result, err := s.resultRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, ErrResultNotFound
	}
	return result, nil
}
