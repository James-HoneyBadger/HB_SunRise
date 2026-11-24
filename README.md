# HB SunRise

## Resurrect your Microsoft Money files on any computer

**No Windows. No copy of Money. No converter. No upload.** HB SunRise reads the
`.mny` database directly — including ledgers written decades ago — and writes its
accounts and transactions in the format you choose.

[![CI](https://github.com/James-HoneyBadger/HB_SunRise/actions/workflows/ci.yml/badge.svg)](https://github.com/James-HoneyBadger/HB_SunRise/actions/workflows/ci.yml)
**Free software** (GPL-3.0-or-later) · **Windows**, **macOS**, **Linux**

HB SunRise is a standalone program for resurrecting archived, obsolete Microsoft
Money files into usable formats. It is built on the engine of
[Honey Ledger](https://github.com/James-HoneyBadger/honey-ledger), extracted into
its own small project. It needs no server, no database and no login, it never
changes the file it reads, and it works fully offline.

## Using it

Double-click the program and answer the questions in the window:

1. the Money file to extract (you can drag the file onto the window),
2. the output formats (one, several, or all),
3. the output folder (Enter keeps the folder of the input file).

On Linux, a file manager may start the program without opening a window. Run it
in a terminal instead, then drag the file onto that terminal to paste its path.

For scripts, give everything on the command line and nothing is asked:

```sh
hb-sunrise -f csv,xlsx -o ./exported "2026_11_21_Backup.mbf"
hb-sunrise -dry-run "2026_11_21_Backup.mbf"   # see what it holds before exporting
```

| Option | Meaning |
|--------|---------|
| `-f`   | Formats, comma separated, or `all` |
| `-o`   | Output folder, created if it does not exist (default: next to the input file) |
| `-dry-run` | Print the account count, transaction count, date range and skipped rows, then exit without writing files |
| `-list` | List the formats and their keys |
| `-version` | Print the version; a downloaded binary reports the release it came from |
| `-h`   | Print the usage summary |

Existing files are never overwritten; a number is added to the new file's name
(`ledger (1).csv`), so running it twice never destroys an earlier export.

The exit status is `0` when every requested file was written, `1` when the file
could not be read, extracted or written, and `2` for a bad option or format name.

## Formats

| Key | File | Use it for |
|-----|------|------------|
| `csv` | `.csv` | Excel and other spreadsheets (UTF-8 with a byte order mark) |
| `honeyledger` | `.honeyledger.csv` | Importing into Honey Ledger without mapping any column |
| `tsv` | `.tsv` | Tab-separated tools (UTF-8 with a byte order mark) |
| `xlsx` | `.xlsx` | An Excel workbook with Transactions and Accounts sheets |
| `json` | `.json` | Scripts and other programs |
| `qif` | `.qif` | Quicken and most finance applications |

In every format except `honeyledger`, amounts are signed from the account's point
of view: money coming in is positive, money going out or moving away is negative.
A transfer is listed once, in the account the money left, with the receiving
account in the *Transfer Account* column. In QIF it appears in both accounts so
the accounts balance.

Text fields (memos, names, categories) that start with a formula character
(`=`, `+`, `-`, `@`) get a leading apostrophe in the CSV, TSV and Honey Ledger
CSV outputs, so a memo cannot run as a formula when the file is opened in a
spreadsheet. XLSX cells are stripped of the control characters a cell cannot
hold; QIF memos are written on one line.

## What is left out

Investment activity, the total line of a split transaction, and rows without an
amount, account or date cannot become ordinary transactions. The program reports
how many rows were left out, so the count is never hidden.

## Building it

```sh
go build -o hb-sunrise ./cmd/hb-sunrise
scripts/build-hb-sunrise.sh   # Linux, Windows and macOS builds in dist/
```

`scripts/build-hb-sunrise.sh` produces the same five files in `dist/` with CGO
disabled, so the result is a single file with no dependencies. It stamps the
version from `package.json`, so those builds report the same number as the
release; a plain `go build` reports `dev`.

```sh
go test ./...
```

The engine tests that need a real Microsoft Money file skip unless
`MSISAM_SAMPLE_FILE` points at a `.mny` file.

## Where this comes from

HB SunRise is extracted from Honey Ledger, a local-first personal finance desktop
app. See [docs/HONEY_LEDGER_REVIEW.md](docs/HONEY_LEDGER_REVIEW.md) for a review
of that project and of the HB Money Extractor program this code is derived from,
and [docs/BUILDING_HB_SUNRISE.md](docs/BUILDING_HB_SUNRISE.md) for the roadmap
from this extractor to the full HB SunRise vision — reconstructing Microsoft
Money files, not only reading them.
