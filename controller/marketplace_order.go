package controller

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const (
	marketplaceOrderInvalidMessage     = "Invalid marketplace order request"
	marketplaceOrderConflictMessage    = "Marketplace records changed. Refresh and try again."
	marketplaceOrderLoadFailureMessage = "Unable to load marketplace order"
	marketplaceOrderFailureMessage     = "Unable to save marketplace order"
)

type marketplaceOrderRequest struct {
	OrderedIDs []int `json:"ordered_ids" binding:"required,max=10000,dive,gt=0"`
}

func GetModelOrder(c *gin.Context) {
	items, err := model.GetModelOrderItems()
	if err != nil {
		common.ApiErrorMsg(c, marketplaceOrderLoadFailureMessage)
		return
	}
	common.ApiSuccess(c, items)
}

func UpdateModelOrder(c *gin.Context) {
	var request marketplaceOrderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, marketplaceOrderInvalidMessage)
		return
	}
	if err := model.ReorderModels(request.OrderedIDs); err != nil {
		writeMarketplaceOrderError(c, err)
		return
	}
	model.RefreshPricing()
	common.ApiSuccess(c, nil)
}

func GetVendorOrder(c *gin.Context) {
	items, err := model.GetVendorOrderItems()
	if err != nil {
		common.ApiErrorMsg(c, marketplaceOrderLoadFailureMessage)
		return
	}
	common.ApiSuccess(c, items)
}

func UpdateVendorOrder(c *gin.Context) {
	var request marketplaceOrderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, marketplaceOrderInvalidMessage)
		return
	}
	if err := model.ReorderVendors(request.OrderedIDs); err != nil {
		writeMarketplaceOrderError(c, err)
		return
	}
	model.RefreshPricing()
	common.ApiSuccess(c, nil)
}

func writeMarketplaceOrderError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrMarketplaceOrderInvalid):
		common.ApiErrorMsg(c, marketplaceOrderInvalidMessage)
	case errors.Is(err, model.ErrMarketplaceOrderConflict):
		common.ApiErrorMsg(c, marketplaceOrderConflictMessage)
	default:
		common.ApiErrorMsg(c, marketplaceOrderFailureMessage)
	}
}
