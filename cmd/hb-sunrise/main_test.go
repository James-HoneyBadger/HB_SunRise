// SPDX-FileCopyrightText: 2020-2026 Honey Badger Universe
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/James-HoneyBadger/hb-sunrise/pkg/converters/msmoney"
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

// TestCleanPathKeepsExistingNamesUntouched guards against cleanPath rewriting a
// path into a different file that happens to exist: a file whose name really
// contains a backslash and a space, or surrounding quote characters, must be
// selected, not mangled.
func TestCleanPathKeepsExistingNamesUntouched(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("backslash and quote characters cannot be part of a Windows file name")
	}

	dir := t.TempDir()

	backslashName := filepath.Join(dir, `odd\ name.mny`)
	require.NoError(t, os.WriteFile(backslashName, []byte("x"), 0o644))
	assert.Equal(t, backslashName, cleanPath(backslashName))

	quotedName := filepath.Join(dir, `"quoted".mny`)
	require.NoError(t, os.WriteFile(quotedName, []byte("x"), 0o644))
	assert.Equal(t, quotedName, cleanPath(quotedName))
}

// TestCleanPathUnescapesWhenOnlyTheUnescapedPathExists checks the drag-and-drop
// case the transformation exists for: the terminal inserts "My\ File.mny" and
// only "My File.mny" is on disk. On Windows a backslash is a separator, not an
// escape, so the transformation is skipped there by design.
func TestCleanPathUnescapesWhenOnlyTheUnescapedPathExists(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a backslash-space is a Windows path separator, not an escape")
	}

	dir := t.TempDir()
	real := filepath.Join(dir, "My File.mny")
	require.NoError(t, os.WriteFile(real, []byte("x"), 0o644))

	assert.Equal(t, real, cleanPath(filepath.Join(dir, `My\ File.mny`)))
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

	extractData = func(ctx msmoney.Context, data []byte) (*msmoney.ExtractedData, error) {
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

func TestRunPrintsValidationReport(t *testing.T) {
	input := filepath.Join(t.TempDir(), "sample.mny")
	require.NoError(t, os.WriteFile(input, []byte("dummy"), 0o644))
	defer stubExtract()()

	duplicate := sampleData()
	duplicate.Transactions = append(duplicate.Transactions, duplicate.Transactions[0])
	extractData = func(ctx msmoney.Context, data []byte) (*msmoney.ExtractedData, error) {
		return duplicate, nil
	}

	var stdout, stderr bytes.Buffer
	assert.Equal(t, 0, run([]string{"-validate", "-dry-run", input}, strings.NewReader(""), &stdout, &stderr))
	assert.Contains(t, stdout.String(), "Validation: 1 confirmed")
	assert.Contains(t, stdout.String(), "duplicate_transaction")
}

func TestRunPrintsDecodingDiagnostics(t *testing.T) {
	input := filepath.Join(t.TempDir(), "sample.mny")
	require.NoError(t, os.WriteFile(input, []byte("dummy"), 0o644))
	defer stubExtract()()

	damaged := sampleData()
	damaged.Diagnostics = msmoney.ExtractedDiagnostics{UnresolvedRows: 3, Samples: []string{"row (definition page 4, page 9, slot 0): row slot cannot be resolved"}}
	extractData = func(ctx msmoney.Context, data []byte) (*msmoney.ExtractedData, error) {
		return damaged, nil
	}

	var stdout, stderr bytes.Buffer
	assert.Equal(t, 0, run([]string{"-dry-run", input}, strings.NewReader(""), &stdout, &stderr))
	assert.Contains(t, stdout.String(), "Decoding problems: 0 rows undecodable, 3 rows unresolved")
}
