// SPDX-FileCopyrightText: 2020-2026 Honey Badger Universe
// SPDX-License-Identifier: GPL-3.0-or-later

// HB SunRise resurrects a Microsoft Money data file (.mny) or backup (.mbf):
// it reads the accounts and transactions the file holds and writes them in the
// formats the user selects. It is a standalone program: it needs no Honey
// Ledger server, database or login, and it never changes the file it reads.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/James-HoneyBadger/hb-sunrise/pkg/converters/msmoney"
	"github.com/James-HoneyBadger/hb-sunrise/pkg/core"
	"github.com/James-HoneyBadger/hb-sunrise/pkg/log"
)

const appName = "HB SunRise"

// version is set at build time from the git tag the commit sits on by
// scripts/build-hb-sunrise.sh, so a downloaded binary and the release it
// came from report the same number. A plain "go build" leaves it as dev.
var version = "dev"

// extractData parses the input file. It is a variable so tests can stand in
// for the parser, which needs a real Microsoft Money data file.
var extractData = msmoney.ExtractMicrosoftMoneyData

func main() {
	log.SetQuiet()

	// The extraction reports its diagnostics through the same logger as the
	// rest of the program, so SetQuiet silences those lines too.
	msmoney.BindLog(msmoney.Logger{
		Errorf: func(ctx msmoney.Context, format string, args ...any) {
			log.Errorf(sunriseContext(ctx), format, args...)
		},
		Infof: func(ctx msmoney.Context, format string, args ...any) {
			log.Infof(sunriseContext(ctx), format, args...)
		},
		Warnf: func(ctx msmoney.Context, format string, args ...any) {
			log.Warnf(sunriseContext(ctx), format, args...)
		},
	})

	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// sunriseContext narrows the engine's context back to the context type of this
// program's log package. The program always passes its own context, so the
// assertion holds; anything else logs without a context.
func sunriseContext(ctx msmoney.Context) core.Context {
	if ctx == nil {
		return nil
	}

	if sunriseContext, ok := ctx.(core.Context); ok {
		return sunriseContext
	}

	return nil
}

func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("hb-sunrise", flag.ContinueOnError)
	flags.SetOutput(stderr)

	formatList := flags.String("f", "", "output format(s), comma separated: "+formatKeys()+", or all")
	outputDir := flags.String("o", "", "output folder (default: next to the input file)")
	dryRun := flags.Bool("dry-run", false, "print the accounts, transactions, date range and skipped rows, and exit without writing files")
	validate := flags.Bool("validate", false, "print the validation report for the extracted data")
	listFormats := flags.Bool("list", false, "list the supported output formats and exit")
	showVersion := flags.Bool("version", false, "print the version and exit")

	flags.Usage = func() {
		fmt.Fprintf(stderr, "%s %s\n\n", appName, version)
		fmt.Fprintf(stderr, "Extracts a Microsoft Money data file (.mny) or backup (.mbf).\n\n")
		fmt.Fprintf(stderr, "Usage:\n  hb-sunrise [options] [file.mny|file.mbf]\n\n")
		fmt.Fprintf(stderr, "Run without a file (or without -f) to be asked what to extract.\n\nOptions:\n")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}

		return 2
	}

	if *showVersion {
		fmt.Fprintf(stdout, "%s %s\n", appName, version)
		return 0
	}

	if *listFormats {
		printFormats(stdout)
		return 0
	}

	reader := bufio.NewReader(stdin)
	interactive := flags.NArg() == 0 || *formatList == ""
	inputPath := ""

	if flags.NArg() > 1 {
		fmt.Fprintln(stderr, "Give one input file at a time.")
		return 2
	} else if flags.NArg() == 1 {
		inputPath = flags.Arg(0)
	}

	selected, err := parseFormats(*formatList)

	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	if interactive {
		fmt.Fprintf(stdout, "%s %s\n\n", appName, version)
	}

	if inputPath == "" {
		inputPath = ask(reader, stdout, "Microsoft Money file to extract (.mny or .mbf): ")
	}

	inputPath = cleanPath(inputPath)

	if inputPath == "" {
		fmt.Fprintln(stderr, "No file selected.")
		return finish(interactive, reader, stdout, 2)
	}

	data, err := os.ReadFile(inputPath)

	if err != nil {
		fmt.Fprintf(stderr, "Cannot read %q: %v\n", inputPath, err)
		return finish(interactive, reader, stdout, 1)
	}

	fmt.Fprintf(stdout, "Reading %s ...\n", filepath.Base(inputPath))

	extracted, err := extractData(core.NewNullContext(), data)

	if err != nil {
		fmt.Fprintf(stderr, "Cannot extract %q: %s\n", inputPath, describeError(err))
		return finish(interactive, reader, stdout, 1)
	}

	printSummary(stdout, extracted)

	if *validate {
		printValidation(stdout, extracted)
	}

	if *dryRun {
		return 0
	}

	if len(extracted.Transactions) < 1 && len(extracted.Accounts) < 1 {
		fmt.Fprintln(stderr, "The file holds no accounts or transactions to extract.")
		return finish(interactive, reader, stdout, 1)
	}

	if len(selected) < 1 {
		selected = chooseFormats(reader, stdout)
	}

	targetDir := *outputDir

	if targetDir == "" && interactive {
		targetDir = cleanPath(ask(reader, stdout, "Output folder [same folder as the input file]: "))
	}

	if targetDir == "" {
		targetDir = filepath.Dir(inputPath)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "Cannot create the output folder %q: %v\n", targetDir, err)
		return finish(interactive, reader, stdout, 1)
	}

	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	failed := false

	for _, selectedFormat := range selected {
		path, err := writeFormat(selectedFormat, targetDir, baseName, extracted)

		if err != nil {
			fmt.Fprintf(stderr, "  %-22s FAILED: %v\n", selectedFormat.name, err)
			failed = true
			continue
		}

		fmt.Fprintf(stdout, "  %-22s %s\n", selectedFormat.name, path)
	}

	if failed {
		return finish(interactive, reader, stdout, 1)
	}

	fmt.Fprintln(stdout, "\nDone.")

	return finish(interactive, reader, stdout, 0)
}

// finish keeps the window open when the program was started interactively —
// by double-clicking it, or with missing arguments — so the messages, success
// or error, can be read before the window closes. A double-clicked program has
// no terminal, so the pause must not depend on stdin being one. Scripted runs
// pass -f and a file and are never paused.
func finish(interactive bool, reader *bufio.Reader, stdout io.Writer, code int) int {
	if interactive {
		ask(reader, stdout, "\nPress Enter to exit. ")
	}

	return code
}

func printSummary(stdout io.Writer, data *msmoney.ExtractedData) {
	fmt.Fprintf(stdout, "Found %d accounts and %d transactions", len(data.Accounts), len(data.Transactions))

	if len(data.Transactions) > 0 {
		first, last := data.Transactions[0].Time, data.Transactions[0].Time

		for _, transaction := range data.Transactions {
			if transaction.Time.Before(first) {
				first = transaction.Time
			}

			if transaction.Time.After(last) {
				last = transaction.Time
			}
		}

		fmt.Fprintf(stdout, " (%s to %s)", first.Format("2006-01-02"), last.Format("2006-01-02"))
	}

	fmt.Fprintln(stdout, ".")

	if data.Skipped > 0 {
		fmt.Fprintf(stdout, "%d rows were left out: investment activity, split totals, and rows without an amount, account or date.\n", data.Skipped)
	}

	if !data.Diagnostics.IsEmpty() {
		fmt.Fprintf(stdout, "Decoding problems: %s.\n", data.Diagnostics.Summary())
	}

	fmt.Fprintln(stdout)
}

func chooseFormats(reader *bufio.Reader, stdout io.Writer) []outputFormat {
	for {
		fmt.Fprintln(stdout, "Output formats:")
		printFormats(stdout)
		fmt.Fprintf(stdout, "  %d) All of the above\n\n", len(allFormats)+1)

		answer := ask(reader, stdout, "Choose one or more, separated by commas [1]: ")

		if answer == "" {
			return []outputFormat{allFormats[0]}
		}

		var chosen []outputFormat
		valid := true

		for _, item := range strings.Split(answer, ",") {
			number, err := strconv.Atoi(strings.TrimSpace(item))

			if err != nil || number < 1 || number > len(allFormats)+1 {
				fmt.Fprintf(stdout, "%q is not one of the choices.\n\n", strings.TrimSpace(item))
				valid = false
				break
			}

			if number == len(allFormats)+1 {
				return allFormats
			}

			chosen = append(chosen, allFormats[number-1])
		}

		if valid && len(chosen) > 0 {
			return chosen
		}
	}
}

func ask(reader *bufio.Reader, stdout io.Writer, prompt string) string {
	fmt.Fprint(stdout, prompt)

	line, _ := reader.ReadString('\n')

	return strings.TrimSpace(line)
}

// cleanPath removes what a shell or file manager adds when a file is dragged
// onto the program: surrounding single or double quotes, surrounding and
// trailing whitespace, and the backslash-escaped spaces a Unix terminal
// inserts. Windows paths keep their backslashes. A transformation is only
// applied when the path as received does not exist, so a real file whose name
// contains quotes or a backslash is still addressed, not silently replaced by
// a different file.
func cleanPath(path string) string {
	path = strings.TrimSpace(path)

	quoted := len(path) >= 2 && (path[0] == '"' || path[0] == '\'') && path[len(path)-1] == path[0]

	if quoted && !pathExists(path) {
		path = strings.TrimSpace(path[1 : len(path)-1])
	}

	if !isWindowsPath(path) && !pathExists(path) {
		path = unescapeSpaces(path)
	}

	return path
}

// pathExists reports whether the path names an existing file or directory, so
// cleanPath leaves a real name alone.
func pathExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

// isWindowsPath reports whether the path is a Windows drive path or a UNC
// path, whose backslashes are separators and must not be treated as escapes.
func isWindowsPath(path string) bool {
	if len(path) >= 3 && path[1] == ':' && path[2] == '\\' &&
		((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) {
		return true
	}

	return strings.HasPrefix(path, `\\`)
}

// unescapeSpaces turns the backslash-escaped spaces a Unix terminal inserts
// when a file is dragged onto it back into plain spaces, so "/path/My\
// File.mny" names the file it points at. A backslash in front of another
// backslash escapes a real backslash, which is kept.
func unescapeSpaces(path string) string {
	var builder strings.Builder
	builder.Grow(len(path))

	for i := 0; i < len(path); i++ {
		switch {
		case path[i] == '\\' && i+1 < len(path) && path[i+1] == ' ':
			builder.WriteByte(' ')
			i++
		case path[i] == '\\' && i+1 < len(path) && path[i+1] == '\\':
			builder.WriteByte('\\')
			i++
		default:
			builder.WriteByte(path[i])
		}
	}

	return builder.String()
}

// printValidation writes the validation report for the extracted data, so a
// user can see what the data holds beyond the export formats.
func printValidation(stdout io.Writer, data *msmoney.ExtractedData) {
	report := msmoney.ValidateTransactions(data)

	if report.IsClean() {
		fmt.Fprintln(stdout, "Validation found nothing to report.")

		return
	}

	fmt.Fprintf(stdout, "Validation: %d confirmed, %d uncertain, %d informational findings\n", len(report.Confirmed()), len(report.Uncertain()), len(report.Findings)-len(report.Confirmed())-len(report.Uncertain()))

	for _, finding := range report.Findings {
		fmt.Fprintf(stdout, "  [%s] %s: %s\n", finding.Severity, finding.Kind, finding.Message)
	}
}

// describeError explains a failure in terms of the file the user picked.
func describeError(err error) string {
	switch {
	case errors.Is(err, msmoney.ErrFileTypeNotSupported):
		return "this is not a Microsoft Money data file (.mny) or backup (.mbf)"
	case errors.Is(err, msmoney.ErrDataFileNotReadable):
		return "the file is damaged, or is not a complete Microsoft Money database"
	case errors.Is(err, msmoney.ErrDataFileEncrypted):
		return "the file is password protected; remove the password in Microsoft Money and try again"
	case errors.Is(err, msmoney.ErrDataFileUnsupportedVersion):
		return "the file was written by Microsoft Money 2000 or earlier, whose layout is not supported; export it with a newer Money first"
	case errors.Is(err, msmoney.ErrBackupFileNotReadable):
		return "the file is damaged, or is not a complete Microsoft Money backup"
	case errors.Is(err, msmoney.ErrBackupFileTooLarge):
		return "the file is too large to extract"
	}

	return err.Error()
}
