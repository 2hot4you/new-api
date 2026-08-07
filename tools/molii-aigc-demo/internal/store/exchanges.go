package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Store) AppendExchange(ctx context.Context, exchange Exchange) (Exchange, error) {
	if strings.TrimSpace(exchange.RunID) == "" || strings.TrimSpace(exchange.Kind) == "" || exchange.StartedAt.IsZero() {
		return Exchange{}, fmt.Errorf("exchange run, kind, and start time are required: %w", ErrInvalidInput)
	}
	for label, value := range map[string][]byte{
		"request headers":  exchange.RequestHeadersJSON,
		"request body":     exchange.RequestBodyJSON,
		"response headers": exchange.ResponseHeadersJSON,
		"response body":    exchange.ResponseBodyJSON,
	} {
		if err := optionalJSON(label, value); err != nil {
			return Exchange{}, err
		}
	}
	if exchange.DurationMS != nil && *exchange.DurationMS < 0 {
		return Exchange{}, ErrInvalidInput
	}
	if exchange.DurationMS == nil && exchange.FinishedAt != nil {
		duration := exchange.FinishedAt.Sub(exchange.StartedAt).Milliseconds()
		if duration < 0 {
			return Exchange{}, fmt.Errorf("exchange finish precedes start: %w", ErrInvalidInput)
		}
		exchange.DurationMS = &duration
	}
	if exchange.ID == "" {
		exchange.ID = uuid.NewString()
	}
	exchange.CreatedAt = s.now()

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if exchange.Sequence <= 0 {
			var maximum int
			if err := tx.Model(&Exchange{}).Where("run_id = ?", exchange.RunID).Select("COALESCE(MAX(sequence), 0)").Scan(&maximum).Error; err != nil {
				return err
			}
			exchange.Sequence = maximum + 1
		}
		return tx.Create(&exchange).Error
	})
	if err != nil {
		return Exchange{}, normalizeDatabaseError(err)
	}
	return exchange, nil
}

func (s *Store) ListExchanges(ctx context.Context, runID string) ([]Exchange, error) {
	var exchanges []Exchange
	if err := s.db.WithContext(ctx).Where("run_id = ?", runID).Order("sequence ASC").Find(&exchanges).Error; err != nil {
		return nil, err
	}
	return exchanges, nil
}

func (s *Store) GetRunWithExchanges(ctx context.Context, runID string) (RunWithExchanges, error) {
	run, err := s.GetRun(ctx, runID)
	if err != nil {
		return RunWithExchanges{}, err
	}
	exchanges, err := s.ListExchanges(ctx, runID)
	if err != nil {
		return RunWithExchanges{}, err
	}
	return RunWithExchanges{Run: run, Exchanges: exchanges}, nil
}
