# Building HB SunRise

How HB SunRise is built, using Honey Ledger's **HB Money Extractor** as the
template. See [HONEY_LEDGER_REVIEW.md](HONEY_LEDGER_REVIEW.md) for the review of
the source project.

## The vision

HB SunRise resurrects archived, obsolete Microsoft Money files:

1. **Extract** — read `.mny` data files and `.mbf` backups and write their
   accounts and transactions as CSV, TSV, XLSX, JSON and QIF, plus a Honey
   Ledger-ready CSV. *This is implemented in this repository.*
2. **Reconstruct** — the reverse direction: read data files of other types
   (QIF, OFX, CSV, …) and *write* a Microsoft Money `.mny` file, recovering
   transaction history that has been locked in dead formats. *This is the next
   phase.*
3. **Anywhere** — Windows, macOS, Linux, as a single dependency-free binary
   (and eventually a desktop window), never uploading data anywhere.

## What was extracted and how

The scaffold mirrors the extractor's two-tier design:

```
cmd/hb-sunrise/                  user interface: CLI + interactive Q&A
  main.go                        flags, prompts, error translation, exit codes
  formats.go                     format registry; writers come from pkg/converters/export
  main_test.go                   tests over a shared sample ExtractedData
  pipeline_test.go               end-to-end tests over synthetic .mny/.mbf files
pkg/converters/msmoney/          the parser (no I/O, no UI knowledge)
  microsoft_money_extract.go     public API: ExtractMicrosoftMoneyData(ctx, data)
  microsoft_money_data_reader.go accounts, categories, transactions, splits
  microsoft_money_backup_archive.go  .mbf cabinet handling with resource limits
  microsoft_money_data_file_signature.go  MSISAM header check
  msisam/                        the engine: header, pages, definitions, rows
  synthetic/                     synthetic .mny/.mbf files for tests
pkg/converters/export/           the six output writers
pkg/core/                        Context interface + NullContext
pkg/log/                         quiet-able logger (stdlib), bound to the extraction
scripts/build-hb-sunrise.sh      static cross-builds for 5 platforms
```

The parser (`msmoney`), the engine (`msisam`), the writers (`export`) and the
synthetic files are self-contained in this repository, so the program builds
with nothing else present.

Rules that keep this working:

- The parser takes `[]byte` and returns plain structs. It never opens files,
  prints, or knows about output formats. Any future GUI reuses it unchanged.
- The parser reports through its `Context` interface and the logger bound in
  `main.go`; `log.SetQuiet()` silences those lines with everything else.
- The parser returns stable sentinel errors; only `main.go` turns them into the
  user-facing messages. Keep that translation table current when errors are
  added.
- Output files are never overwritten (`uniquePath` adds ` (n)`), and skipped
  rows are always counted and reported.
- Copied code keeps its GPL-3.0-or-later SPDX headers; new files get the same.

## Extending the scaffold

### Add an output format

1. Add a writer to [`pkg/converters/export`](../pkg/converters/export), then an
   entry to `allFormats` in [formats.go](../cmd/hb-sunrise/formats.go): key,
   name, extension, description, and a `write func(io.Writer,
   *msmoney.ExtractedData) error` calling it.
2. Follow the amount rules: signed from the account's point of view everywhere
   except `honeyledger`; transfers listed once with the receiving account, and
   in both accounts for QIF.
3. Add a test next to the writer in `export`; the end-to-end pipeline test
   covers the new format for free.

### Add an input reader

Add a package with the same shape as `msmoney`:
`Extract<Format>Data(ctx, data []byte) (*ExtractedData, error)`. If it can be
expressed with the same `ExtractedAccount` / `ExtractedTransaction` structs,
every output format works for it for free, so just register the new reader in
`main.go`.

## The roadmap

### Phase 1 — extraction (done)

Read `.mny`/`.mbf`, write the six formats, interactive and scriptable, tested,
static cross-builds. Everything in this repository today.

### Phase 2 — reconstructing a `.mny` file

The MSISAM engine in `pkg/converters/msmoney/msisam` is read-only. Writing a
Money file means adding the inverse of each reader:

- **Header and key block.** Honey Ledger's `docs/MONEY_FILE_FORMAT.md`
  documents the header, the scrambled block that hides the database key, and
  the RC4 + page layout. A writer must produce a header Money will accept and
  scramble the key block the same way (`database.go` currently unscrambles it).
- **Table definitions.** Money expects specific tables (`ACCT`, `CAT`, `TRN`,
  `TRN_SPLIT`, `CRNC`, …) with specific column names, types and sizes. The
  definition writer is the inverse of `definition.go`.
- **Rows and pages.** Allocate data pages, fill slots, and write the row layout
  (`row.go`, `value.go`) with Money's encodings (UTF-16 text, date/time, fixed
  point amounts at 4 decimal places — see `microsoftMoneyAmountScale`).
- **Acceptance tests.** Reconstruct, then read the result back with the
  existing parser and compare; then open the result in Money Plus Sunset on
  Windows for the real verdict.

Work package order suggestion: single-table database (definitions only) →
`ACCT` + `CRNC` → `CAT` → `TRN` (no splits) → `TRN_SPLIT` → `.mbf` cabinet
writing (same `go-cabinet` library, inverse direction).

### Phase 3 — importing other formats

Readers for QIF, OFX/QFX, CSV and TSV feeding the same intermediate
`ExtractedData`/transaction model, so *any input* can go to *any output*,
including the `.mny` writer. Honey Ledger already has reference implementations
for QIF/OFX/CSV import in its `pkg/converters/` to consult under the GPL.

### Phase 4 — the desktop application

The extractor's interactive Q&A works but is terminal-bound. Options, in order
of effort:

1. A small local web UI (Go serves one HTML page; the existing `run()` logic
   becomes a library call) — easiest way to get drag-and-drop.
2. Electron + the compiled binary as a child process — what Honey Ledger
   itself does, heaviest but closest to a polished app.

### Phase 5 — release pipeline

`scripts/build-hb-sunrise.sh` produces static binaries for five platforms, and
`.github/workflows/release.yml` runs it on every `v*` tag, verifies the
stamped version and generates `SHA256SUMS.txt`. The same workflow builds the
Wails desktop application on Linux, Windows and macOS (Apple silicon and
Intel), so every release carries the desktop build next to the CLI binaries and
the checksums — the same pattern Honey Ledger uses for the extractor. The
macOS build is published as a zipped `.app`; the desktop builds are unsigned,
like the CLI binaries.

## Testing strategy

- **Synthetic, committed.** The format writers are tested against
  `sampleData()` (see `main_test.go`). Add byte-level fixtures for the MSISAM
  writer as it lands.
- **Real files, environment-gated.** Like Honey Ledger, tests that need an
  actual `.mny` read `MSISAM_SAMPLE_FILE` and skip otherwise, because the
  format cannot be redistributed.
- **Round-trip.** The Phase 2 acceptance test is `read(write(x)) == x` for a
  well-chosen sample covering every column type, every transaction type,
  transfers, splits, and empty tables.

## Desktop application

The graphical application lives in [`desktop/`](../desktop): a Wails v2 +
React + TypeScript app binding the same engine the CLI uses.

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
cd desktop
wails generate module     # refresh frontend/wailsjs bindings after changing app.go
wails build -tags webkit2_41   # Linux, webkit2gtk-4.1; binary in build/bin/
wails dev                 # hot-reload development with the vite server
```

Requirements: Go, Node.js, and on Linux `libgtk-3-dev libwebkit2gtk-4.1-dev`.
The build produces a single self-contained binary; end users need nothing
installed. CI builds the desktop application on every push.

## Conventions

- Module: `github.com/James-HoneyBadger/hb-sunrise`. License: GPL-3.0-or-later.
- The git tag is the single source of version truth: the build script stamps
  the tag it is built from into `main.version` via
  `git describe --tags --exact-match`; any other build reports `dev`.
- Exit codes: `0` success, `1` file could not be read/extracted/written, `2`
  bad option or format name.
- Never overwrite an existing output file; never hide skipped rows.
