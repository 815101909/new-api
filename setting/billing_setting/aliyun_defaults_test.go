package billing_setting

import "testing"

func TestAliyunDefaultBillingExprsSmoke(t *testing.T) {
	for model, expr := range GetDefaultBillingExprMap() {
		if err := SmokeTestExpr(expr); err != nil {
			t.Fatalf("model %s billing expr failed smoke test: %v", model, err)
		}
	}
}
