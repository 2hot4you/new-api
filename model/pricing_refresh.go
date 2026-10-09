package model

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
	updatePricingLock.Lock()
	defer updatePricingLock.Unlock()

	modelSupportEndpointsLock.Lock()
	defer modelSupportEndpointsLock.Unlock()

	updatePricing()
}
