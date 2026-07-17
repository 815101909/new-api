package billing_setting

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
)

func TestAliyunDefaultBillingExprsSmoke(t *testing.T) {
	for model, expr := range GetDefaultBillingExprMap() {
		if err := SmokeTestExpr(expr); err != nil {
			t.Fatalf("model %s billing expr failed smoke test: %v", model, err)
		}
	}
}

func TestBillingDefaultsFallbackAfterPartialDatabaseLoad(t *testing.T) {
	saved := map[string]string{}
	if err := config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	t.Cleanup(func() {
		if err := config.GlobalConfig.LoadFromDB(saved); err != nil {
			t.Fatalf("restore config failed: %v", err)
		}
	})

	if err := config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"custom-model":"ratio"}`,
		"billing_setting.billing_expr": `{"custom-model":"tier(\"custom\", p * 1)"}`,
	}); err != nil {
		t.Fatalf("load config failed: %v", err)
	}

	if mode := GetBillingMode("paraformer-v2"); mode != BillingModeTieredExpr {
		t.Fatalf("paraformer-v2 billing mode = %s, want %s", mode, BillingModeTieredExpr)
	}

	expr, ok := GetBillingExpr("paraformer-v2")
	if !ok || strings.TrimSpace(expr) == "" {
		t.Fatalf("paraformer-v2 billing expr missing after partial database load")
	}
}
