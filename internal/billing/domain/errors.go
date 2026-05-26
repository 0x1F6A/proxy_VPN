package domain

import "errors"

var (
	ErrPlanNotFound     = errors.New("plan not found")
	ErrPackNotFound     = errors.New("data pack not found")
	ErrCouponNotFound   = errors.New("coupon not found")
	ErrCouponExpired    = errors.New("coupon expired or not active")
	ErrCouponExhausted  = errors.New("coupon usage limit reached")
	ErrCouponNotMet     = errors.New("order amount below coupon minimum")
	ErrCouponWrongScope = errors.New("coupon not applicable to this order")
	ErrOrderNotFound    = errors.New("order not found")
	ErrOrderNotPayable  = errors.New("order not payable (status or expired)")
	ErrOrderConflict    = errors.New("idempotency key already used by a different order")
	ErrInvalidType      = errors.New("invalid order type")
	ErrPlanInactive     = errors.New("plan not active")
	ErrPackInactive     = errors.New("data pack not active")
)
