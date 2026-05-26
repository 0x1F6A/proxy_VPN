package gormrepo

import (
	"testing"

	"github.com/0x1F6A/proxy_VPN/internal/payment/domain"
)

// pmToRow must coerce empty decimal fields to "0" so MySQL decimal columns
// don't error with "Incorrect decimal value: ''".
func TestPmToRow_EmptyDecimalsCoercedToZero(t *testing.T) {
	row := pmToRow(&domain.Payment{
		OrderNo:     "X1",
		UserID:      1,
		Channel:     domain.ChannelAlipay,
		AmountCNY:   "",
		AmountToken: "",
		Status:      domain.StatusPending,
	})
	if row.AmountCNY != "0" {
		t.Fatalf("AmountCNY: want 0, got %q", row.AmountCNY)
	}
	if row.AmountToken != "0" {
		t.Fatalf("AmountToken: want 0, got %q", row.AmountToken)
	}
}

func TestPmToRow_PreservesNonEmptyDecimals(t *testing.T) {
	row := pmToRow(&domain.Payment{
		AmountCNY:   "9.90",
		AmountToken: "1.234567",
	})
	if row.AmountCNY != "9.90" || row.AmountToken != "1.234567" {
		t.Fatalf("got %+v", row)
	}
}
