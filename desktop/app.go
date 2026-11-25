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
)

const maxRecentFiles = 10

// accountView is one account as the window shows it.
type accountView struct {
	Name     string `json:"name"`
	Currency string `json:"currency"`
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
	File         fileInfoView                 `json:"file"`
	Accounts     []accountView                `json:"accounts"`
	Transactions []transactionView            `json:"transactions"`
	Skipped      int                          `json:"skipped"`
	Diagnostics  msmoney.ExtractedDiagnostics `json:"diagnostics"`
	Validation   []findingView                `json:"validation"`
	Elapsed      string                       `json:"elapsed"`
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
	ctx     context.Context
	session *extractionView
	cancel  context.CancelFunc
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

	hash := sha256.Sum256(data)

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

	view := &extractionView{
		File: fileInfoView{
			Path:   path,
			Name:   filepath.Base(path),
			Size:   int64(len(data)),
			Sha256: hex.EncodeToString(hash[:]),
		},
		Accounts:     make([]accountView, 0, len(extracted.Accounts)),
		Transactions: make([]transactionView, 0, len(extracted.Transactions)),
		Skipped:      extracted.Skipped,
		Diagnostics:  extracted.Diagnostics,
		Validation:   make([]findingView, 0, len(report.Findings)),
		Elapsed:      time.Since(start).Round(time.Millisecond).String(),
	}

	for _, account := range extracted.Accounts {
		view.Accounts = append(view.Accounts, accountView{Name: account.Name, Currency: account.Currency})
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
		})
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

	a.session = view
	a.addRecent(path)

	return view, nil
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

func currencyOf(accounts []msmoney.ExtractedAccount, name string) string {
	for _, account := range accounts {
		if account.Name == name {
			return account.Currency
		}
	}

	return ""
}

// RecentFiles returns the most recently opened files, newest first.
func (a *App) RecentFiles() []string {
	path, err := recentStore()

	if err != nil {
		return nil
	}

	content, err := os.ReadFile(path)

	if err != nil {
		return nil
	}

	recent := make([]string, 0, maxRecentFiles)

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
