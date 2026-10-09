package model

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// RefreshPricing 强制立即重新计算与定价相关的缓存。
// 该方法用于需要最新数据的内部管理 API，
// 因此会绕过默认的 1 分钟延迟刷新。
func RefreshPricing() {
	_ = WithCatalogReadBarrier(func() error {
		refreshPricingGuarded()
		return nil
	})
}

// The caller already holds a catalog read or write barrier. Never reacquire it
// here: a waiting writer makes recursive RWMutex read locking unsafe.
func refreshPricingGuarded() {
	if err := refreshCatalogPricingGuarded(context.Background(), nil); err != nil {
		common.SysLog("refresh pricing: " + err.Error())
	}
}

// Required lifecycle rebuild errors must reach the publisher before it can
// acknowledge a durable revision. No compiler runs when stage is supplied.
func refreshCatalogPricingGuarded(ctx context.Context, stage *catalogRuntimeStage) error {
	if stage != nil {
		if !updatePricingLock.TryLock() {
			return ErrCatalogWriterBusy
		}
	} else {
		updatePricingLock.Lock()
	}
	defer updatePricingLock.Unlock()

	if stage != nil {
		if !modelSupportEndpointsLock.TryLock() {
			return ErrCatalogWriterBusy
		}
	} else {
		modelSupportEndpointsLock.Lock()
	}
	defer modelSupportEndpointsLock.Unlock()
	if stage != nil {
		pricingMap, vendorsList, lastGetPricingTime = nil, nil, time.Time{}
		ratio_setting.InvalidateExposedDataCache()
	}

	db := DB
	if stage != nil {
		db = db.WithContext(ctx)
	}
	return updatePricingWithCatalogStage(db, stage)
}
