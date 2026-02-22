package util

import (
	sdkmath "cosmossdk.io/math"
)

const (
	DmExponent       = 9
	UsdScaleExponent = 6
)

var (
	DmScale      = sdkmath.NewIntWithDecimal(1, DmExponent)
	UsdScale     = sdkmath.NewIntWithDecimal(1, UsdScaleExponent)
	UsdFrom18To6 = sdkmath.NewInt(1_000_000_000_000)
	UsdExponent  = sdkmath.NewIntWithDecimal(1, 18)
)

func PtrInt(val int64) *sdkmath.Int {
	i := sdkmath.NewInt(val)
	return &i
}
