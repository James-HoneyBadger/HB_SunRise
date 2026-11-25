// SPDX-FileCopyrightText: 2020-2026 Honey Badger Universe
// SPDX-License-Identifier: GPL-3.0-or-later

// Package main is the HB SunRise desktop application: the graphical client of
// the extraction engine. The engine itself lives in
// github.com/James-HoneyBadger/hb-sunrise/pkg/converters and is shared with
// the command line tool in cmd/hb-sunrise; this package only binds it to the
// window.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/James-HoneyBadger/hb-sunrise/pkg/converters/export"
	"github.com/James-HoneyBadger/hb-sunrise/pkg/converters/msmoney"
	"github.com/James-HoneyBadger/hb-sunrise/pkg/converters/msmoney/msisam"
)

const maxRecentFiles = 10

// accountView is one account as the window shows it.
type accountView struct {
	Name     string `json:"name"`
	Currency string `json:"currency"`
	Closed   bool   `json:"closed"`
	Hidden   bool   `json:"hidden"`
}

// transactionView is one transaction as the window shows it. The amount is in
// hundredths; the window formats it.
type transactionView struct {
	Time            string `json:"time"`
	Type            string `json:"type"`
	Account         string `json:"account"`
	Currency        string `json:"currency"`
	Amount          int64  `json:"amount"`
	TransferAccount string `json:"transferAccount"`
	TransferAmount  int64  `json:"transferAmount"`
	Category        string `json:"category"`
	SubCategory     string `json:"subCategory"`
	Memo            string `json:"memo"`
	Payee           string `json:"payee"`
	Split           bool   `json:"split"`
}

// findingView is one validation finding as the window shows it.
type findingView struct {
	Severity        string `json:"severity"`
	Kind            string `json:"kind"`
	Account         string `json:"account"`
	TransferAccount string `json:"transferAccount"`
	Time            string `json:"time"`
	Amount          int64  `json:"amount"`
	Message         string `json:"message"`
}

// fileInfoView describes the file an extraction came from, for provenance.
type fileInfoView struct {
	Path   string `json:"path"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Sha256 string `json:"sha256"`
}

// extractionView is everything the window shows for one opened file.
type extractionView struct {
	File              fileInfoView                 `json:"file"`
	Accounts          []accountView                `json:"accounts"`
	Transactions      []transactionView            `json:"transactions"`
	Categories        []categorySummaryView        `json:"categories"`
	Payees            []payeeSummaryView           `json:"payees"`
	Transfers         []transferView               `json:"transfers"`
	Splits            []splitView                  `json:"splits"`
	Skipped           int                          `json:"skipped"`
	Diagnostics       msmoney.ExtractedDiagnostics `json:"diagnostics"`
	Validation        []findingView                `json:"validation"`
	ValidationSummary validationSummaryView        `json:"validationSummary"`
	Elapsed           string                       `json:"elapsed"`
}

// categorySummaryView aggregates the transactions of one category path.
type categorySummaryView struct {
	Path    string `json:"path"`
	Count   int    `json:"count"`
	Expense int64  `json:"expense"`
	Income  int64  `json:"income"`
}

// payeeSummaryView aggregates the transactions of one payee.
type payeeSummaryView struct {
	Name    string `json:"name"`
	Count   int    `json:"count"`
	Expense int64  `json:"expense"`
	Income  int64  `json:"income"`
}

// validationSummaryView carries the report's calculated counts and totals for
// the validation workspace.
type validationSummaryView struct {
	TransactionCount int    `json:"transactionCount"`
	IncomeCount      int    `json:"incomeCount"`
	ExpenseCount     int    `json:"expenseCount"`
	TransferCount    int    `json:"transferCount"`
	IncomeTotal      int64  `json:"incomeTotal"`
	ExpenseTotal     int64  `json:"expenseTotal"`
	Earliest         string `json:"earliest"`
	Latest           string `json:"latest"`
}

// transferView is one transfer as the window shows it, with both sides.
type transferView struct {
	Date            string `json:"date"`
	Account         string `json:"account"`
	TransferAccount string `json:"transferAccount"`
	Amount          int64  `json:"amount"`
	TransferAmount  int64  `json:"transferAmount"`
	// Matched reports whether the receiving side's amount was matched from
	// its own row; when false, the source amount was used for both sides.
	Matched bool   `json:"matched"`
	Memo    string `json:"memo"`
}

// splitView is one split fragment as the window shows it.
type splitView struct {
	Date        string `json:"date"`
	Account     string `json:"account"`
	Category    string `json:"category"`
	SubCategory string `json:"subCategory"`
	Amount      int64  `json:"amount"`
	Memo        string `json:"memo"`
}

// recentStore is the location of the recent files list.
func recentStore() (string, error) {
	dir, err := os.UserConfigDir()

	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "hb-sunrise", "recent.txt"), nil
}

// appContext adapts a cancellable context to the engine's context interface,
// so a Cancel from the window stops the extraction between rows.
type appContext struct {
	context.Context
}

func (c appContext) GetContextId() string {
	return "desktop"
}

func (c appContext) GetClientLocale() string {
	return ""
}

// App is the bound backend of the window.
type App struct {
	ctx        context.Context
	session    *extractionView
	cancel     context.CancelFunc
	inspection *inspectionSession
}

// NewApp returns the application the window binds.
func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// OpenFileDialog asks the user for a Microsoft Money file with the native
// dialog and returns the chosen path, or an empty string when cancelled.
func (a *App) OpenFileDialog() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open a Microsoft Money file",
		Filters: []runtime.FileFilter{
			{DisplayName: "Microsoft Money (*.mny, *.mbf)", Pattern: "*.mny;*.mbf"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	})

	return path, err
}

// OpenPath reads, extracts and validates the Money file at path, reporting
// progress to the window as it goes. The previous session is replaced.
func (a *App) OpenPath(path string) (*extractionView, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("no file selected")
	}

	start := time.Now()
	data, err := os.ReadFile(path)

	if err != nil {
		return nil, fmt.Errorf("cannot read the file: %w", err)
	}

	cancellable, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	defer cancel()

	ctx := appContext{Context: cancellable}

	extracted, err := msmoney.ExtractMicrosoftMoneyDataWithOptions(ctx, data, msmoney.ExtractionOptions{
		Progress: func(progress msmoney.Progress) {
			runtime.EventsEmit(a.ctx, "extract-progress", progress)
		},
	})

	if err != nil {
		if ctx.Err() != nil {
			return nil, errors.New("extraction cancelled")
		}

		return nil, describeExtractionError(err)
	}

	report := msmoney.ValidateTransactions(extracted)
	view := buildExtractionView(path, data, extracted, report, start)

	a.session = view
	a.addRecent(path)

	return view, nil
}

// buildExtractionView turns an extracted Money database into everything the
// window shows: file provenance, accounts, transactions, and the derived
// categories, transfers, and splits workspaces.
func buildExtractionView(path string, data []byte, extracted *msmoney.ExtractedData, report msmoney.ValidationReport, start time.Time) *extractionView {
	hash := sha256.Sum256(data)

	view := &extractionView{
		File: fileInfoView{
			Path:   path,
			Name:   filepath.Base(path),
			Size:   int64(len(data)),
			Sha256: hex.EncodeToString(hash[:]),
		},
		Accounts:     make([]accountView, 0, len(extracted.Accounts)),
		Transactions: make([]transactionView, 0, len(extracted.Transactions)),
		Categories:   make([]categorySummaryView, 0),
		Transfers:    make([]transferView, 0),
		Splits:       make([]splitView, 0),
		Skipped:      extracted.Skipped,
		Diagnostics:  extracted.Diagnostics,
		Validation:   make([]findingView, 0, len(report.Findings)),
		Elapsed:      time.Since(start).Round(time.Millisecond).String(),
	}

	for _, account := range extracted.Accounts {
		view.Accounts = append(view.Accounts, accountView{
			Name:     account.Name,
			Currency: account.Currency,
			Closed:   account.Closed,
			Hidden:   account.Hidden,
		})
	}

	for _, transaction := range extracted.Transactions {
		view.Transactions = append(view.Transactions, transactionView{
			Time:            transaction.Time.Format("2006-01-02"),
			Type:            transaction.Type,
			Account:         transaction.Account,
			Currency:        transaction.Currency,
			Amount:          transaction.Amount,
			TransferAccount: transaction.TransferAccount,
			TransferAmount:  transaction.TransferAmount,
			Category:        transaction.Category,
			SubCategory:     transaction.SubCategory,
			Memo:            transaction.Memo,
			Payee:           transaction.Payee,
			Split:           transaction.Split,
		})
	}

	categoryIndex := make(map[string]int, 16)

	for _, transaction := range extracted.Transactions {
		path := categoryPathOf(transaction)

		if path == "" {
			continue
		}

		index, exists := categoryIndex[path]

		if !exists {
			view.Categories = append(view.Categories, categorySummaryView{Path: path})
			index = len(view.Categories) - 1
			categoryIndex[path] = index
		}

		view.Categories[index].Count++

		if transaction.Type == msmoney.ExtractedTypeExpense {
			view.Categories[index].Expense += transaction.Amount
		} else if transaction.Type == msmoney.ExtractedTypeIncome {
			view.Categories[index].Income += transaction.Amount
		}
	}

	for _, transaction := range extracted.Transactions {
		if transaction.Type == msmoney.ExtractedTypeTransfer {
			view.Transfers = append(view.Transfers, transferView{
				Date:            transaction.Time.Format("2006-01-02"),
				Account:         transaction.Account,
				TransferAccount: transaction.TransferAccount,
				Amount:          transaction.Amount,
				TransferAmount:  transaction.TransferAmount,
				Matched:         transaction.TransferAmount != 0,
				Memo:            transaction.Memo,
			})
		}

		if transaction.Split {
			view.Splits = append(view.Splits, splitView{
				Date:        transaction.Time.Format("2006-01-02"),
				Account:     transaction.Account,
				Category:    transaction.Category,
				SubCategory: transaction.SubCategory,
				Amount:      transaction.Amount,
				Memo:        transaction.Memo,
			})
		}
	}

	sort.Slice(view.Categories, func(left, right int) bool { return view.Categories[left].Path < view.Categories[right].Path })

	payeeIndex := make(map[string]int, 16)

	for _, transaction := range extracted.Transactions {
		if transaction.Payee == "" {
			continue
		}

		index, exists := payeeIndex[transaction.Payee]

		if !exists {
			view.Payees = append(view.Payees, payeeSummaryView{Name: transaction.Payee})
			index = len(view.Payees) - 1
			payeeIndex[transaction.Payee] = index
		}

		view.Payees[index].Count++

		if transaction.Type == msmoney.ExtractedTypeExpense {
			view.Payees[index].Expense += transaction.Amount
		} else if transaction.Type == msmoney.ExtractedTypeIncome {
			view.Payees[index].Income += transaction.Amount
		}
	}

	sort.Slice(view.Payees, func(left, right int) bool { return view.Payees[left].Name < view.Payees[right].Name })

	view.ValidationSummary = validationSummaryView{
		TransactionCount: report.Summary.TransactionCount,
		IncomeCount:      report.Summary.IncomeCount,
		ExpenseCount:     report.Summary.ExpenseCount,
		TransferCount:    report.Summary.TransferCount,
		IncomeTotal:      report.Summary.IncomeTotal,
		ExpenseTotal:     report.Summary.ExpenseTotal,
	}

	if !report.Summary.Earliest.IsZero() {
		view.ValidationSummary.Earliest = report.Summary.Earliest.Format("2006-01-02")
		view.ValidationSummary.Latest = report.Summary.Latest.Format("2006-01-02")
	}

	for _, finding := range report.Findings {
		view.Validation = append(view.Validation, findingView{
			Severity:        string(finding.Severity),
			Kind:            string(finding.Kind),
			Account:         finding.Account,
			TransferAccount: finding.TransferAccount,
			Time:            finding.Time.Format("2006-01-02"),
			Amount:          finding.Amount,
			Message:         finding.Message,
		})
	}

	return view
}

// Cancel stops the extraction that is in progress, when one is.
func (a *App) Cancel() {
	if a.cancel != nil {
		a.cancel()
	}
}

// SaveFileDialog asks the user where to write an export with the native
// dialog and returns the chosen path, or an empty string when cancelled.
func (a *App) SaveFileDialog(key string) (string, error) {
	format, err := export.FormatByKey(key)

	if err != nil {
		return "", err
	}

	if a.session == nil {
		return "", errors.New("open a file first")
	}

	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export " + format.Name,
		DefaultFilename: strings.TrimSuffix(a.session.File.Name, filepath.Ext(a.session.File.Name)) + format.Extension,
	})

	return path, err
}

// Export writes the open file's data in the given format to the given path.
// An existing file is never overwritten.
func (a *App) Export(path string, key string) error {
	if a.session == nil {
		return errors.New("open a file first")
	}

	if _, err := os.Stat(path); err == nil {
		return errors.New("the file already exists; choose another name")
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)

	if err != nil {
		return err
	}

	defer file.Close()

	return export.Write(key, file, "HB SunRise", a.sessionData())
}

// data rebuilds the extracted data the session was built from, so exports do
// not need to keep the engine's structs around.
func (a *App) sessionData() *msmoney.ExtractedData {
	if a.session == nil {
		return &msmoney.ExtractedData{}
	}

	data := &msmoney.ExtractedData{
		Accounts:     make([]msmoney.ExtractedAccount, 0, len(a.session.Accounts)),
		Transactions: make([]msmoney.ExtractedTransaction, 0, len(a.session.Transactions)),
	}

	for _, account := range a.session.Accounts {
		data.Accounts = append(data.Accounts, msmoney.ExtractedAccount{Name: account.Name, Currency: account.Currency})
	}

	for _, transaction := range a.session.Transactions {
		when, _ := time.Parse("2006-01-02", transaction.Time)

		data.Transactions = append(data.Transactions, msmoney.ExtractedTransaction{
			Time:             when,
			Type:             transaction.Type,
			Account:          transaction.Account,
			Currency:         transaction.Currency,
			Amount:           transaction.Amount,
			TransferAccount:  transaction.TransferAccount,
			TransferCurrency: currencyOf(data.Accounts, transaction.TransferAccount),
			TransferAmount:   transaction.TransferAmount,
			Category:         transaction.Category,
			SubCategory:      transaction.SubCategory,
			Memo:             transaction.Memo,
		})
	}

	return data
}

// categoryPathOf joins a transaction's category and subcategory the way the
// report shows them.
func categoryPathOf(transaction msmoney.ExtractedTransaction) string {
	if transaction.Category != "" && transaction.SubCategory != "" {
		return transaction.Category + " / " + transaction.SubCategory
	}

	if transaction.SubCategory != "" {
		return transaction.SubCategory
	}

	return transaction.Category
}

func currencyOf(accounts []msmoney.ExtractedAccount, name string) string {
	for _, account := range accounts {
		if account.Name == name {
			return account.Currency
		}
	}

	return ""
}

// RecentFiles returns the most recently opened files, newest first. It returns
// an empty list, never nil: a nil Go slice becomes JSON null, and the window
// reads the list before it can render anything.
func (a *App) RecentFiles() []string {
	recent := make([]string, 0, maxRecentFiles)

	path, err := recentStore()

	if err != nil {
		return recent
	}

	content, err := os.ReadFile(path)

	if err != nil {
		return recent
	}

	for _, line := range strings.Split(string(content), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			recent = append(recent, line)
		}
	}

	return recent
}

// ClearRecentFiles removes the recent files list.
func (a *App) ClearRecentFiles() error {
	path, err := recentStore()

	if err != nil {
		return err
	}

	return os.Remove(path)
}

func (a *App) addRecent(path string) {
	recent := a.RecentFiles()
	filtered := make([]string, 0, len(recent)+1)
	filtered = append(filtered, path)

	for _, existing := range recent {
		if existing != path {
			filtered = append(filtered, existing)
		}
	}

	if len(filtered) > maxRecentFiles {
		filtered = filtered[:maxRecentFiles]
	}

	store, err := recentStore()

	if err != nil {
		return
	}

	if err := os.MkdirAll(filepath.Dir(store), 0o700); err != nil {
		return
	}

	sort.Strings(filtered[1:])

	_ = os.WriteFile(store, []byte(strings.Join(filtered, "\n")+"\n"), 0o600)
}

// describeExtractionError explains an extraction failure in the window's
// words, matching the command line tool's phrasing.
func describeExtractionError(err error) error {
	switch {
	case errors.Is(err, msmoney.ErrFileTypeNotSupported):
		return errors.New("this is not a Microsoft Money data file (.mny) or backup (.mbf)")
	case errors.Is(err, msmoney.ErrDataFileNotReadable):
		return errors.New("the file is damaged, or is not a complete Microsoft Money database")
	case errors.Is(err, msmoney.ErrDataFileEncrypted):
		return errors.New("the file is password protected; remove the password in Microsoft Money and try again")
	case errors.Is(err, msmoney.ErrDataFileUnsupportedVersion):
		return errors.New("the file was written by Microsoft Money 2000 or earlier, whose layout is not supported; export it with a newer Money first")
	case errors.Is(err, msmoney.ErrBackupFileNotReadable):
		return errors.New("the backup is damaged, or is not a complete Microsoft Money backup")
	case errors.Is(err, msmoney.ErrBackupFileTooLarge):
		return errors.New("the backup is too large to extract")
	default:
		return err
	}
}

// Inspection

type pageTypeCount struct {
	Type  int    `json:"type"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type columnView struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Fixed  bool   `json:"fixed"`
	Number int    `json:"number"`
	Offset int    `json:"offset"`
	Size   int    `json:"size"`
}

type tableView struct {
	Name           string       `json:"name"`
	DefinitionPage int          `json:"definitionPage"`
	RowCount       int          `json:"rowCount"`
	FirstDataPage  int          `json:"firstDataPage"`
	Columns        []columnView `json:"columns"`
}

type inspectionView struct {
	File           fileInfoView    `json:"file"`
	Version        int             `json:"version"`
	CodePage       int             `json:"codePage"`
	LanguageID     int             `json:"languageID"`
	PageCount      int             `json:"pageCount"`
	PageTypeCounts []pageTypeCount `json:"pageTypeCounts"`
	Tables         []tableView     `json:"tables"`
}

type fieldView struct {
	Column string `json:"column"`
	Type   string `json:"type"`
	Value  string `json:"value"`
	Null   bool   `json:"null"`
}

type rowsView struct {
	Table        string                       `json:"table"`
	Offset       int                          `json:"offset"`
	Limit        int                          `json:"limit"`
	DeclaredRows int                          `json:"declaredRows"`
	Rows         [][]fieldView                `json:"rows"`
	Diagnostics  msmoney.ExtractedDiagnostics `json:"diagnostics"`
}

// inspectionSession keeps the bytes and page view of the file being inspected.
type inspectionSession struct {
	data     []byte
	database *msisam.Database
}

// Inspect opens the file at path for raw inspection: header, pages and table
// schemas, without walking any rows. It replaces the inspection session.
func (a *App) Inspect(path string) (*inspectionView, error) {
	data, err := os.ReadFile(path)

	if err != nil {
		return nil, fmt.Errorf("cannot read the file: %w", err)
	}

	database, err := msisam.Open(data)

	if err != nil {
		kind := msmoney.ErrDataFileNotReadable

		if errors.Is(err, msisam.ErrEncryptedDatabase) {
			kind = msmoney.ErrDataFileEncrypted
		} else if errors.Is(err, msisam.ErrUnsupportedVersion) {
			kind = msmoney.ErrDataFileUnsupportedVersion
		}

		return nil, describeExtractionError(fmt.Errorf("%w: %v", kind, err))
	}

	a.inspection = &inspectionSession{data: data, database: database}

	hash := sha256.Sum256(data)

	view := &inspectionView{
		File: fileInfoView{
			Path:   path,
			Name:   filepath.Base(path),
			Size:   int64(len(data)),
			Sha256: hex.EncodeToString(hash[:]),
		},
		Version:        int(database.Version()),
		CodePage:       int(database.CodePage()),
		LanguageID:     int(database.LanguageID()),
		PageCount:      database.PageCount(),
		PageTypeCounts: make([]pageTypeCount, 0, 6),
		Tables:         make([]tableView, 0),
	}

	for name, count := range database.PageTypeCounts() {
		view.PageTypeCounts = append(view.PageTypeCounts, pageTypeCount{Type: int(name), Name: pageTypeName(name), Count: count})
	}

	sort.Slice(view.PageTypeCounts, func(left, right int) bool {
		return view.PageTypeCounts[left].Type < view.PageTypeCounts[right].Type
	})

	tables, err := database.Tables()

	if err != nil {
		return nil, fmt.Errorf("cannot read the table catalog: %w", err)
	}

	for _, table := range tables {
		entry := tableView{
			Name:           table.Name,
			DefinitionPage: table.Definition.Page,
			RowCount:       table.Definition.RowCount,
			FirstDataPage:  table.Definition.FirstDataPage,
			Columns:        make([]columnView, 0, len(table.Definition.Columns)),
		}

		for _, column := range table.Definition.Columns {
			entry.Columns = append(entry.Columns, columnView{
				Name:   column.Name,
				Type:   column.Type.String(),
				Fixed:  column.Fixed,
				Number: column.Number,
				Offset: column.Offset,
				Size:   column.Size,
			})
		}

		view.Tables = append(view.Tables, entry)
	}

	return view, nil
}

// InspectRows returns a page of raw records of the given table, decoded into
// human-readable values. The record walk honours Cancel. Only the page is
// collected, so a large table stays responsive.
func (a *App) InspectRows(tableName string, offset int, limit int) (*rowsView, error) {
	if a.inspection == nil {
		return nil, errors.New("inspect a file first")
	}

	if offset < 0 {
		offset = 0
	}

	if limit < 1 || limit > 500 {
		limit = 100
	}

	database := a.inspection.database
	table, err := database.Table(tableName)

	if err != nil {
		return nil, fmt.Errorf("no table named %q: %w", tableName, err)
	}

	diagnostics := &msisam.Diagnostics{}
	database.SetDiagnostics(diagnostics)

	cancellable, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	defer cancel()

	ctx := appContext{Context: cancellable}

	result := &rowsView{
		Table:        table.Name,
		Offset:       offset,
		Limit:        limit,
		DeclaredRows: table.Definition.RowCount,
		Rows:         make([][]fieldView, 0, limit),
	}

	skipped := 0
	errRowsCollected := errors.New("page collected")

	err = database.EachRow(ctx, table.Definition, func(fields []msisam.Field) error {
		if skipped < offset {
			skipped++

			return nil
		}

		if len(result.Rows) >= limit {
			return errRowsCollected
		}

		row := make([]fieldView, 0, len(fields))

		for _, field := range fields {
			row = append(row, fieldView{
				Column: field.Column.Name,
				Type:   field.Column.Type.String(),
				Value:  renderFieldValue(database, field),
				Null:   field.Null && field.Column.Type != msisam.ColumnTypeBool,
			})
		}

		result.Rows = append(result.Rows, row)

		return nil
	})

	if err != nil && !errors.Is(err, errRowsCollected) {
		if ctx.Err() != nil {
			return nil, errors.New("inspection cancelled")
		}

		return nil, fmt.Errorf("cannot read rows: %w", err)
	}

	result.Diagnostics = diagnosticsView(diagnostics)

	return result, nil
}

// diagnosticsView converts the engine's decoding diagnostics into the public
// report shape the window already knows.
func diagnosticsView(diagnostics *msisam.Diagnostics) msmoney.ExtractedDiagnostics {
	if diagnostics == nil || diagnostics.IsEmpty() {
		return msmoney.ExtractedDiagnostics{}
	}

	result := msmoney.ExtractedDiagnostics{
		CorruptRows:           diagnostics.CorruptRows,
		UnresolvedRows:        diagnostics.UnresolvedRows,
		CorruptDefinitions:    diagnostics.CorruptDefinitions,
		UnknownCatalogEntries: diagnostics.UnknownCatalogEntries,
		Samples:               make([]string, 0, len(diagnostics.RowSamples)+len(diagnostics.DefinitionSamples)),
	}

	for _, sample := range diagnostics.RowSamples {
		result.Samples = append(result.Samples, fmt.Sprintf("row (definition page %d, page %d, slot %d): %s", sample.Definition, sample.Page, sample.Slot, sample.Err))
	}

	for _, sample := range diagnostics.DefinitionSamples {
		result.Samples = append(result.Samples, fmt.Sprintf("definition (page %d): %s", sample.Page, sample.Err))
	}

	return result
}

// renderFieldValue renders one decoded field for the inspection workspace.
// Money is shown as its stored integer and its decimal value, so the raw scale
// is never hidden behind a floating point rendering.
func renderFieldValue(database *msisam.Database, field msisam.Field) string {
	if field.Column.Type == msisam.ColumnTypeBool {
		if database.Bool(field) {
			return "true"
		}

		return "false"
	}

	if field.Null {
		return ""
	}

	switch field.Column.Type {
	case msisam.ColumnTypeByte, msisam.ColumnTypeInt, msisam.ColumnTypeLongInt:
		value, ok := database.Int(field)

		if !ok {
			return "(cannot decode)"
		}

		return fmt.Sprintf("%d", value)
	case msisam.ColumnTypeMoney:
		value, ok := database.Int(field)

		if !ok {
			return "(cannot decode)"
		}

		return fmt.Sprintf("%d (%.4f)", value, float64(value)/10000)
	case msisam.ColumnTypeFloat, msisam.ColumnTypeDouble:
		value, ok := database.Float(field)

		if !ok {
			return "(cannot decode)"
		}

		return fmt.Sprintf("%g", value)
	case msisam.ColumnTypeNumeric:
		value, ok := database.Numeric(field)

		if !ok {
			return "(cannot decode)"
		}

		return fmt.Sprintf("%g", value)
	case msisam.ColumnTypeDateTime:
		value, ok := database.Time(field)

		if !ok {
			return "(cannot decode)"
		}

		return value.Format("2006-01-02 15:04:05")
	case msisam.ColumnTypeText, msisam.ColumnTypeMemo:
		value, err := database.String(field)

		if err != nil {
			return "(cannot decode)"
		}

		return value
	case msisam.ColumnTypeGUID:
		value, ok := database.GUID(field)

		if !ok {
			return "(cannot decode)"
		}

		return value
	case msisam.ColumnTypeBinary, msisam.ColumnTypeOLE:
		value, err := database.Bytes(field)

		if err != nil {
			return "(cannot decode)"
		}

		if len(value) > 64 {
			return fmt.Sprintf("%d bytes: %x…", len(value), value[:32])
		}

		return fmt.Sprintf("%x", value)
	default:
		return fmt.Sprintf("(type %s)", field.Column.Type)
	}
}

func pageTypeName(pageType byte) string {
	switch pageType {
	case msisam.PageTypeHeader:
		return "header"
	case msisam.PageTypeData:
		return "data"
	case msisam.PageTypeDefinition:
		return "definition"
	case msisam.PageTypeIndex:
		return "index"
	case msisam.PageTypeLeaf:
		return "leaf"
	case msisam.PageTypeUsageMap:
		return "usage map"
	default:
		return fmt.Sprintf("unknown(0x%02X)", pageType)
	}
}
