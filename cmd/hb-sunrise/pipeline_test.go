// SPDX-FileCopyrightText: 2020-2026 Honey Badger Universe
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/James-HoneyBadger/hb-sunrise/pkg/converters/msmoney/synthetic"
)

// TestEndToEndPipelineWithSyntheticMoneyFile runs the complete extraction
// pipeline on a synthesized Microsoft Money data file: parse the binary
// format, then write every output format.
func TestEndToEndPipelineWithSyntheticMoneyFile(t *testing.T) {
	dir := t.TempDir()

	input := filepath.Join(dir, "sample.mny")
	require.NoError(t, os.WriteFile(input, synthetic.MoneyFile(), 0o644))
	outDir := filepath.Join(dir, "out")

	var stdout, stderr bytes.Buffer

	assert.Equal(t, 0, run([]string{"-f", "all", "-o", outDir, input}, strings.NewReader(""), &stdout, &stderr))

	for _, name := range []string{"sample.csv", "sample.honeyledger.csv", "sample.tsv", "sample.xlsx", "sample.json", "sample.qif"} {
		assert.FileExists(t, filepath.Join(outDir, name), "format %s must be written", name)
	}

	assert.Contains(t, stdout.String(), "Found 2 accounts and 3 transactions")
	assert.Contains(t, stdout.String(), "3 rows were left out")
	assert.Contains(t, stdout.String(), "Done.")

	// Spot check the content of every format.
	csv := readFile(t, filepath.Join(outDir, "sample.csv"))
	assert.Contains(t, csv, "Checking")
	assert.Contains(t, csv, "Food,Groceries")
	assert.Contains(t, csv, "-30.00")

	tsv := readFile(t, filepath.Join(outDir, "sample.tsv"))
	assert.Contains(t, tsv, "Food\tGroceries")

	honeyledger := readFile(t, filepath.Join(outDir, "sample.honeyledger.csv"))
	assert.Contains(t, honeyledger, "Transfer")
	assert.Contains(t, honeyledger, "Savings")

	qif := readFile(t, filepath.Join(outDir, "sample.qif"))
	assert.Contains(t, qif, "!Account")
	assert.Contains(t, qif, "NChecking")
	assert.Contains(t, qif, "L[Savings]")
	assert.Contains(t, qif, "T100.00")

	var document struct {
		Source   string `json:"source"`
		Accounts []struct {
			Name     string `json:"name"`
			Currency string `json:"currency"`
		} `json:"accounts"`
		Transactions []struct {
			Type            string `json:"type"`
			Account         string `json:"account"`
			Amount          string `json:"amount"`
			Category        string `json:"category"`
			Subcategory     string `json:"subcategory"`
			TransferAccount string `json:"transferAccount"`
			Memo            string `json:"memo"`
		} `json:"transactions"`
		Skipped int `json:"skippedRows"`
	}

	require.NoError(t, json.Unmarshal([]byte(readFile(t, filepath.Join(outDir, "sample.json"))), &document))
	assert.Equal(t, 2, len(document.Accounts))
	assert.Equal(t, 3, len(document.Transactions))
	assert.Equal(t, 3, document.Skipped)
	assert.Equal(t, "Groceries", document.Transactions[0].Subcategory)
	assert.Equal(t, "Savings", document.Transactions[2].TransferAccount)

	workbook, err := excelize.OpenFile(filepath.Join(outDir, "sample.xlsx"))
	require.NoError(t, err)

	defer workbook.Close()

	transactions, err := workbook.GetRows("Transactions")
	require.NoError(t, err)
	assert.Equal(t, 4, len(transactions), "header plus three transactions")
	assert.Equal(t, "Food", transactions[1][6])

	accounts, err := workbook.GetRows("Accounts")
	require.NoError(t, err)
	assert.Equal(t, 3, len(accounts), "header plus two accounts")
	assert.Equal(t, "Savings", accounts[2][0])
}

// TestEndToEndPipelineWithSyntheticBackupFile runs the same pipeline on a
// Microsoft Cabinet backup (.mbf) wrapping the same database.
func TestEndToEndPipelineWithSyntheticBackupFile(t *testing.T) {
	dir := t.TempDir()

	input := filepath.Join(dir, "sample.mbf")
	require.NoError(t, os.WriteFile(input, synthetic.BackupFile(), 0o644))
	outDir := filepath.Join(dir, "out")

	var stdout, stderr bytes.Buffer

	assert.Equal(t, 0, run([]string{"-f", "json,csv", "-o", outDir, input}, strings.NewReader(""), &stdout, &stderr))
	assert.FileExists(t, filepath.Join(outDir, "sample.json"))
	assert.FileExists(t, filepath.Join(outDir, "sample.csv"))
	assert.Contains(t, stdout.String(), "Found 2 accounts and 3 transactions")
	assert.Contains(t, stdout.String(), "3 rows were left out")
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(content)
}
