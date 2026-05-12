package billing_setting

func defaultVolcEngineBillingModeMap() map[string]string {
	return map[string]string{
		"doubao-seed-1-6-thinking-250615": BillingModeTieredExpr,
		"doubao-seed-1-6-thinking-250715": BillingModeTieredExpr,
	}
}

func defaultVolcEngineBillingExprMap() map[string]string {
	expr := `len <= 32000 && c <= 200 ? tier("short_discount", p * 0.8 + c * 2 + cr * 0.16) : len <= 32000 ? tier("short", p * 0.8 + c * 8 + cr * 0.16) : len <= 128000 ? tier("mid", p * 1.2 + c * 16 + cr * 0.16) : tier("long", p * 2.4 + c * 24 + cr * 0.16)`
	return map[string]string{
		"doubao-seed-1-6-thinking-250615": expr,
		"doubao-seed-1-6-thinking-250715": expr,
	}
}
