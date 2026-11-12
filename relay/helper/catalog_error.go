package helper

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// IsCatalogPricingUnavailable distinguishes a local publication gate from an
// absent quote. Wrapping must not change the classification.
func IsCatalogPricingUnavailable(err error) bool {
	return errors.Is(err, model.ErrCatalogWriterBusy) || errors.Is(err, model.ErrCatalogPublicationPending)
}

// ModelPricingError preserves the missing-price contract while making local
// catalog contention non-retryable and ineligible for channel auto-disable.
func ModelPricingError(err error) *types.NewAPIError {
	if IsCatalogPricingUnavailable(err) {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeCatalogPricingUnavailable, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
	}
	return types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
}
