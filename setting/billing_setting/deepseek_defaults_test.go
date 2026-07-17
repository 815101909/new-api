package billing_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
)

func TestDeepSeekV4DefaultBilling(t *testing.T) {
	tests := []struct {
		model    string
		expected float64
	}{
		{model: "deepseek-v4-flash", expected: 1004},
		{model: "deepseek-v4-pro", expected: 3005},
	}

	for _, test := range tests {
		t.Run(test.model, func(t *testing.T) {
			if mode := GetBillingMode(test.model); mode != BillingModeTieredExpr {
				t.Fatalf("billing mode = %s, want %s", mode, BillingModeTieredExpr)
			}
			expr, ok := GetBillingExpr(test.model)
			if !ok {
				t.Fatal("billing expression is missing")
			}
			result, _, err := billingexpr.RunExpr(expr, billingexpr.TokenParams{
				P:  800,
				C:  100,
				CR: 200,
			})
			if err != nil {
				t.Fatal(err)
			}
			if result != test.expected {
				t.Fatalf("billing result = %f, want %f", result, test.expected)
			}
		})
	}
}
