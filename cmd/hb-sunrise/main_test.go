// SPDX-FileCopyrightText: 2020-2026 Honey Badger Universe
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/James-HoneyBadger/hb-sunrise/pkg/converters/msmoney"
	"github.com/James-HoneyBadger/hb-sunrise/pkg/core"
)

func sampleData() *msmoney.ExtractedData {
	day := time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC)

	return &msmoney.ExtractedData{
		Accounts: []msmoney.ExtractedAccount{{Name: "Checking", Currency: "USD"}, {Name: "Savings", Currency: "USD"}, {Name: "Unused", Currency: "USD"}},
		Transactions: []msmoney.ExtractedTransaction{
			{Time: day, Type: msmoney.ExtractedTypeExpense, Account: "Checking", Currency: "USD", Amount: 1234, Category: "Food", SubCategory: "Groceries", Memo: "Milk, \"2%\"\nand eggs"},
			{Time: day, Type: msmoney.ExtractedTypeIncome, Account: "Checking", Currency: "USD", Amount: 500000, Category: "Salary", SubCategory: "Pay"},
			{Time: day, Type: msmoney.ExtractedTypeTransfer, Account: "Checking", Currency: "USD", Amount: 10000, TransferAccount: "Savings", TransferCurrency: "USD"},
		},
	}
}

func TestParseFormats(t *testing.T) {
	formats, err := parseFormats("csv, JSON,csv")
	assert.Nil(t, err)
	assert.Equal(t, 2, len(formats))

	formats, err = parseFormats("all")
	assert.Nil(t, err)
	assert.Equal(t, len(allFormats), len(formats))

	_, err = parseFormats("pdf")
	assert.NotNil(t, err)
}

func TestFormatAmount(t *testing.T) {
	assert.Equal(t, "12.34", formatAmount(1234))
	assert.Equal(t, "-0.05", formatAmount(-5))
	assert.Equal(t, "0.00", formatAmount(0))
}

func TestCSVKeepsAmountsSignedAndQuotesText(t *testing.T) {
	var out bytes.Buffer
	assert.Nil(t, writeCSV(&out, sampleData()))

	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(out.String(), "\ufeff"))).ReadAll()
	assert.Nil(t, err)
	assert.Equal(t, 4, len(rows))
	assert.Equal(t, "-12.34", rows[1][4])
	assert.Equal(t, "5000.00", rows[2][4])
	assert.Equal(t, "-100.00", rows[3][4])
	assert.Equal(t, "Savings", rows[3][5])
	assert.Equal(t, "Milk, \"2%\"\nand eggs", rows[1][8])
}

func TestHoneyLedgerCSVHasImportableLayout(t *testing.T) {
	var out bytes.Buffer
	assert.Nil(t, writeHoneyLedgerCSV(&out, sampleData()))

	rows, err := csv.NewReader(&out).ReadAll()
	assert.Nil(t, err)
	assert.Equal(t, "Sub Category", rows[0][4])
	assert.Equal(t, "2024-03-05 00:00:00", rows[1][0])
	assert.Equal(t, "+00:00", rows[1][1])
	assert.Equal(t, "12.34", rows[1][7])
	assert.Equal(t, "Transfer", rows[3][2])
	assert.Equal(t, "Savings", rows[3][8])
	assert.Equal(t, "100.00", rows[3][10])
}

func TestJSONHasAccountsAndTransactions(t *testing.T) {
	var out bytes.Buffer
	assert.Nil(t, writeJSON(&out, sampleData()))

	var document struct {
		Accounts     []map[string]any `json:"accounts"`
		Transactions []map[string]any `json:"transactions"`
	}

	assert.Nil(t, json.Unmarshal(out.Bytes(), &document))
	assert.Equal(t, 3, len(document.Accounts))
	assert.Equal(t, 3, len(document.Transactions))
	assert.Equal(t, "-12.34", document.Transactions[0]["amount"])
}

func TestQIFWritesBothSidesOfATransfer(t *testing.T) {
	var out bytes.Buffer
	assert.Nil(t, writeQIF(&out, sampleData()))
	text := out.String()

	assert.Equal(t, 3, strings.Count(text, "!Account"))
	assert.Contains(t, text, "D03/05/2024\nT-100.00\nL[Savings]\n^")
	assert.Contains(t, text, "D03/05/2024\nT100.00\nL[Checking]\n^")
	assert.Contains(t, text, "LFood:Groceries")
	assert.Contains(t, text, "PMilk, \"2%\" and eggs")
}

func TestCSVNeutralizesFormulaPrefixes(t *testing.T) {
	day := time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC)
	data := &msmoney.ExtractedData{
		Accounts: []msmoney.ExtractedAccount{{Name: "Checking", Currency: "USD"}},
		Transactions: []msmoney.ExtractedTransaction{
			{Time: day, Type: msmoney.ExtractedTypeExpense, Account: "=cmd|' /C calc'!A0", Currency: "USD", Amount: 1234, Category: "+Food", SubCategory: "@Sub", Memo: "=HYPERLINK(\"http://evil\")"},
		},
	}

	var out bytes.Buffer
	assert.Nil(t, writeCSV(&out, data))

	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(out.String(), "\ufeff"))).ReadAll()
	assert.Nil(t, err)
	assert.Equal(t, "'=cmd|' /C calc'!A0", rows[1][2])
	assert.Equal(t, "-12.34", rows[1][4], "amounts keep their sign and are never prefixed")
	assert.Equal(t, "'+Food", rows[1][6])
	assert.Equal(t, "'@Sub", rows[1][7])
	assert.Equal(t, "'=HYPERLINK(\"http://evil\")", rows[1][8])
}

func TestHoneyLedgerCSVNeutralizesFormulaPrefixes(t *testing.T) {
	day := time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC)
	data := &msmoney.ExtractedData{
		Accounts: []msmoney.ExtractedAccount{{Name: "Checking", Currency: "USD"}},
		Transactions: []msmoney.ExtractedTransaction{
			{Time: day, Type: msmoney.ExtractedTypeExpense, Account: "=cmd|' /C calc'!A0", Currency: "USD", Amount: 1234, Category: "+Food", SubCategory: "@Sub", Memo: "=HYPERLINK(\"http://evil\")"},
		},
	}

	var out bytes.Buffer
	assert.Nil(t, writeHoneyLedgerCSV(&out, data))

	rows, err := csv.NewReader(&out).ReadAll()
	assert.Nil(t, err)
	assert.Equal(t, "'+Food", rows[1][3])
	assert.Equal(t, "'@Sub", rows[1][4])
	assert.Equal(t, "'=cmd|' /C calc'!A0", rows[1][5])
	assert.Equal(t, "12.34", rows[1][7], "amounts are never prefixed")
	assert.Equal(t, "'=HYPERLINK(\"http://evil\")", rows[1][13])
}

func TestXLSXStripsInvalidCharacters(t *testing.T) {
	day := time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC)
	data := &msmoney.ExtractedData{
		Accounts: []msmoney.ExtractedAccount{{Name: "Bad\x01Name\uFFFE", Currency: "USD"}},
		Transactions: []msmoney.ExtractedTransaction{
			{Time: day, Type: msmoney.ExtractedTypeExpense, Account: "Checking", Currency: "USD", Amount: 1234, Memo: "line1\x01line2\nline3\uFFFF"},
		},
	}

	var out bytes.Buffer
	assert.Nil(t, writeXLSX(&out, data))

	file, err := excelize.OpenReader(&out)
	assert.Nil(t, err)

	defer file.Close()

	rows, err := file.GetRows("Transactions")
	assert.Nil(t, err)
	assert.Equal(t, "line1line2\nline3", rows[1][8])

	accounts, err := file.GetRows("Accounts")
	assert.Nil(t, err)
	assert.Equal(t, "BadName", accounts[1][0])
}

func TestXLSXHasBothSheets(t *testing.T) {
	var out bytes.Buffer
	assert.Nil(t, writeXLSX(&out, sampleData()))

	file, err := excelize.OpenReader(&out)
	assert.Nil(t, err)

	defer file.Close()

	rows, err := file.GetRows("Transactions")
	assert.Nil(t, err)
	assert.Equal(t, 4, len(rows))

	accounts, err := file.GetRows("Accounts")
	assert.Nil(t, err)
	assert.Equal(t, 4, len(accounts))
}

func TestWriteFormatNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	csvFormat := allFormats[0]

	first, err := writeFormat(csvFormat, dir, "money", sampleData())
	assert.Nil(t, err)
	assert.Equal(t, filepath.Join(dir, "money.csv"), first)

	second, err := writeFormat(csvFormat, dir, "money", sampleData())
	assert.Nil(t, err)
	assert.Equal(t, filepath.Join(dir, "money (1).csv"), second)

	hl, err := writeFormat(allFormats[1], dir, "money", sampleData())
	assert.Nil(t, err)
	hl2, err := writeFormat(allFormats[1], dir, "money", sampleData())
	assert.Nil(t, err)
	assert.Equal(t, filepath.Join(dir, "money.honeyledger.csv"), hl)
	assert.Equal(t, filepath.Join(dir, "money (1).honeyledger.csv"), hl2)
}

func TestRunRejectsInvalidInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	dir := t.TempDir()

	assert.Equal(t, 2, run([]string{"-f", "pdf", "x.mny"}, strings.NewReader(""), &stdout, &stderr))
	assert.Equal(t, 1, run([]string{"-f", "csv", filepath.Join(dir, "missing.mny")}, strings.NewReader(""), &stdout, &stderr))

	notMoney := filepath.Join(dir, "not.mny")
	assert.Nil(t, os.WriteFile(notMoney, []byte("hello"), 0o644))
	stderr.Reset()
	assert.Equal(t, 1, run([]string{"-f", "csv", notMoney}, strings.NewReader(""), &stdout, &stderr))
	assert.Contains(t, stderr.String(), "not a Microsoft Money data file")
}

func TestVersionFlagReportsTheBuildVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	original := version
	version = "2.1.0"

	defer func() { version = original }()

	assert.Equal(t, 0, run([]string{"-version"}, strings.NewReader(""), &stdout, &stderr))
	assert.Equal(t, "HB SunRise 2.1.0\n", stdout.String())
}

func TestCleanPathStripsQuotesWhitespaceAndEscapedSpaces(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"double quotes", `"C:\Money\My File.mny"`, `C:\Money\My File.mny`},
		{"single quotes", `'/home/me/My File.mny'`, `/home/me/My File.mny`},
		{"trailing whitespace", "  /home/me/My File.mny  \n", `/home/me/My File.mny`},
		{"escaped spaces", `/home/me/My\ File.mny`, `/home/me/My File.mny`},
		{"escaped backslash and space", `/home/me/odd\\ file.mny`, `/home/me/odd\ file.mny`},
		{"windows drive path untouched", `C:\Program Files\Money\file.mny`, `C:\Program Files\Money\file.mny`},
		{"unc path untouched", `\\server\share\My File.mny`, `\\server\share\My File.mny`},
		{"unix backslashes kept", `/home/me/keep\back.mny`, `/home/me/keep\back.mny`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, cleanPath(test.in))
		})
	}
}

// panicOnRead fails the test if anything tries to read from it, so a run that
// must stay non-interactive cannot accidentally pause or ask a question.
type panicOnRead struct{}

func (panicOnRead) Read([]byte) (int, error) {
	panic("unexpected stdin read: a scripted run must not pause or ask")
}

// stubExtract returns a restore function that puts the real parser back.
func stubExtract() func() {
	original := extractData

	extractData = func(ctx core.Context, data []byte) (*msmoney.ExtractedData, error) {
		return sampleData(), nil
	}

	return func() { extractData = original }
}

func TestInteractiveErrorPausesBeforeExit(t *testing.T) {
	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader("/nonexistent.mny\n")

	assert.Equal(t, 1, run(nil, stdin, &stdout, &stderr))
	assert.Contains(t, stdout.String(), "Press Enter to exit.")
}

func TestInteractiveSuccessPausesAtEnd(t *testing.T) {
	input := filepath.Join(t.TempDir(), "sample.mny")
	require.NoError(t, os.WriteFile(input, []byte("dummy"), 0o644))
	defer stubExtract()()

	stdin := strings.NewReader(input + "\n\n\n")
	var stdout, stderr bytes.Buffer

	assert.Equal(t, 0, run(nil, stdin, &stdout, &stderr))
	assert.Contains(t, stdout.String(), "Done.")
	assert.Contains(t, stdout.String(), "Press Enter to exit.")
}

func TestScriptedRunNeverPausesOrAsks(t *testing.T) {
	input := filepath.Join(t.TempDir(), "sample.mny")
	require.NoError(t, os.WriteFile(input, []byte("dummy"), 0o644))
	defer stubExtract()()

	outDir := filepath.Join(t.TempDir(), "out")
	var stdout, stderr bytes.Buffer

	assert.Equal(t, 0, run([]string{"-f", "csv", "-o", outDir, input}, panicOnRead{}, &stdout, &stderr))
	assert.NotContains(t, stdout.String(), "Press Enter")
	assert.FileExists(t, filepath.Join(outDir, "sample.csv"))
}

func TestScriptedErrorStillDoesNotPause(t *testing.T) {
	var stdout, stderr bytes.Buffer

	assert.Equal(t, 1, run([]string{"-f", "csv", "/nonexistent.mny"}, panicOnRead{}, &stdout, &stderr))
	assert.NotContains(t, stdout.String(), "Press Enter")
}

func TestDryRunPrintsSummaryAndWritesNothing(t *testing.T) {
	input := filepath.Join(t.TempDir(), "sample.mny")
	require.NoError(t, os.WriteFile(input, []byte("dummy"), 0o644))
	defer stubExtract()()

	var stdout, stderr bytes.Buffer

	assert.Equal(t, 0, run([]string{"-dry-run", input}, panicOnRead{}, &stdout, &stderr))
	assert.Contains(t, stdout.String(), "Found 3 accounts and 3 transactions")
	assert.NotContains(t, stdout.String(), "Press Enter")
	assert.NotContains(t, stdout.String(), "sample.csv")
}
