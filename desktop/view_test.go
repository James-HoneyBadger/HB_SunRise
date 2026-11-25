package main

import (
	"context"
	"testing"
	"time"

	"github.com/James-HoneyBadger/hb-sunrise/pkg/converters/msmoney"
	"github.com/James-HoneyBadger/hb-sunrise/pkg/converters/msmoney/synthetic"
)

// buildTestView extracts the synthetic database and builds the workspace
// views the desktop app derives from it.
func buildTestView(t *testing.T) *extractionView {
	t.Helper()

	data := synthetic.MoneyFile()
	extracted, err := msmoney.ExtractMicrosoftMoneyData(appContext{Context: context.Background()}, data)

	if err != nil {
		t.Fatalf("extract synthetic database: %v", err)
	}

	return buildExtractionView("synthetic.mny", data, extracted, msmoney.ValidateTransactions(extracted), time.Now())
}

func TestBuildExtractionViewProvenance(t *testing.T) {
	view := buildTestView(t)

	if view.File.Name != "synthetic.mny" {
		t.Errorf("file name = %q, want synthetic.mny", view.File.Name)
	}

	if len(view.File.Sha256) != 64 {
		t.Errorf("sha256 = %q, want 64 hex characters", view.File.Sha256)
	}
}

func TestBuildExtractionViewCategories(t *testing.T) {
	view := buildTestView(t)

	if len(view.Categories) == 0 {
		t.Fatal("categories view is empty, want the categorized transactions")
	}

	var counted, expense, income int64

	for _, category := range view.Categories {
		if category.Count <= 0 {
			t.Errorf("category %q count = %d, want positive", category.Path, category.Count)
		}

		counted += int64(category.Count)
		expense += category.Expense
		income += category.Income
	}

	// The synthetic database holds two categorized transactions: groceries
	// and rent. The engine reports expense amounts as positive magnitudes.
	if counted != 2 {
		t.Errorf("categorized transaction count = %d, want 2", counted)
	}

	if expense != 5000 || income != 0 {
		t.Errorf("expense = %d income = %d, want 5000 and 0", expense, income)
	}
}

func TestBuildExtractionViewTransfers(t *testing.T) {
	view := buildTestView(t)

	// Money stores both sides of a transfer; the engine keeps the outgoing
	// side only, with the receiving side's own amount attached.
	if len(view.Transfers) != 1 {
		t.Fatalf("transfers = %d, want 1", len(view.Transfers))
	}

	transfer := view.Transfers[0]

	if transfer.Account != "Checking" || transfer.TransferAccount != "Savings" {
		t.Errorf("transfer accounts = %q -> %q, want Checking -> Savings", transfer.Account, transfer.TransferAccount)
	}

	if transfer.Amount != 10000 || transfer.TransferAmount != 10000 {
		t.Errorf("transfer amounts = %d / %d, want 10000 / 10000", transfer.Amount, transfer.TransferAmount)
	}

	if !transfer.Matched {
		t.Errorf("synthetic transfer should carry a matched receiving amount: %+v", transfer)
	}
}

func TestBuildExtractionViewSplits(t *testing.T) {
	view := buildTestView(t)

	if len(view.Splits) != 2 {
		t.Fatalf("splits = %d, want 2 fragments", len(view.Splits))
	}

	for _, fragment := range view.Splits {
		if fragment.Account == "" {
			t.Errorf("split fragment has empty account: %+v", fragment)
		}
	}
}
