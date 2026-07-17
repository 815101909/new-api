package billing_setting

func defaultDeepSeekBillingExprMap() map[string]string {
	return map[string]string{
		"deepseek-v4-flash": `tier("base", p * 1 + c * 2 + cr * 0.02)`,
		"deepseek-v4-pro":   `tier("base", p * 3 + c * 6 + cr * 0.025)`,
	}
}
