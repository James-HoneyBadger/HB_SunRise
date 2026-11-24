# Review: Honey Ledger and the HB Money Extractor

Review of <https://github.com/James-HoneyBadger/honey-ledger>, written while
extracting its **HB Money Extractor** program as the starting point for HB
SunRise. Reviewed at commit on the default branch, 2026-11-23.

## What Honey Ledger is

Honey Ledger is a standalone Electron desktop app for personal finance
management, released under GPL-3.0-or-later. It is **local-first**: a Go
backend runs as a child process of Electron, serves a Vue 3 + Vuetify SPA over a
custom `app://` protocol, and stores everything in SQLite on the user's machine
(MySQL/PostgreSQL optional). There is no account, no hosted server, and no
telemetry. Its headline feature is reading **Microsoft Money `.mny` files
natively** — a Jet 4 database in the MSISAM variant that Jet/Access drivers,
LibreOffice Base, and every other tool refuse to open.

Key facts from the README:

- Reads `.mny` data files and `.mbf` backups (Microsoft Cabinet archives around
  the same database) directly, in-process, on Windows, macOS and Linux.
- Verified end to end against a real Money Sunset database: 165 accounts, 193
  categories and 39,293 transaction rows, of which 28,057 become transactions.
- Honest about limits: investment activity is skipped, Money 2000 and earlier
  use an older 2048-byte page layout that is not read yet, and a
  password-protected file is reported as exactly that.
- The whole format is documented in `docs/MONEY_FILE_FORMAT.md` — header, the
  scrambled block hiding the database key, page types, table definitions, row
  layout.

## Architecture at a glance

```
Electron shell ──► Vue 3 SPA ──► Gin HTTP API ──► services ──► datastore ──► SQLite
                                                       └──► cron jobs, MCP server
```

- `cmd/` — Go entry points (`webserver`, `database`, `user_data`, `security`,
  `cron`, `utility`, `openapi`, and **`hb-money-extractor`**).
- `pkg/` — domain packages: `models`, `core` (context wrappers), `errs`,
  `services`, `datastore`, `converters` (importers for QIF/OFX/CSV/XLS/Money),
  `mcp`, `settings`, `log`, and more.
- `src/` — the Vue 3 frontend; `electron/` — the desktop shell.
- `docs/` — unusually good documentation, including the proprietary file
  format itself.

The engineering standard is high: extensive tests (including real-file tests
that skip when a sample is not supplied), per-feature error codes with stable
messages, bounded resource limits when parsing untrusted files, and
cross-platform release automation.

## The HB Money Extractor

A standalone program shipped with each Honey Ledger release. It reads a `.mny`
or `.mbf` and writes its accounts and transactions as CSV, Honey Ledger CSV,
TSV, XLSX, JSON or QIF — with no server, database or login.

### Structure

| File | Role |
|------|------|
| `cmd/hb-money-extractor/main.go` | CLI + interactive flow, error translation, exit codes |
| `cmd/hb-money-extractor/formats.go` | The six output writers, one function each |
| `pkg/converters/msmoney/microsoft_money_extract.go` | Public API: `ExtractMicrosoftMoneyData(ctx, data) → ExtractedData` |
| `pkg/converters/msmoney/microsoft_money_data_reader.go` | Reads currencies, accounts, categories, transaction rows; flattens splits and transfers |
| `pkg/converters/msmoney/microsoft_money_backup_archive.go` | Unpacks `.mbf` cabinets in memory, with decompression-bomb limits |
| `pkg/converters/msmoney/msisam/` | The MSISAM engine: header, scrambled key block, page chain, table definitions, row cracking |
| `scripts/build-money-extractor.sh` | Cross-compiles 5 static binaries (linux amd64/arm64, windows amd64, darwin amd64/arm64), stamps version from `package.json` |

### Design decisions worth copying

1. **Two-tier API.** The parser (`msmoney`) knows nothing about files or
   formats; it takes `[]byte` and returns plain structs
   (`ExtractedData`, `ExtractedAccount`, `ExtractedTransaction`). The command
   layer owns I/O, interactivity and format writing. A GUI or a server can
   reuse the parser unchanged.
2. **A context for logging, nothing more.** The parser takes a
   `core.Context` used only by the log package; the extractor passes
   `core.NewNullContext()` and calls `log.SetQuiet()` so library log lines
   never clutter the tool's output.
3. **Errors by code, words at the edge.** The parser returns stable typed
   errors (`errs.ErrImportMicrosoftMoneyDataFileEncrypted`, ...); `main.go`
   translates them into the three troubleshooting messages documented for
   users. Tests and UIs can match on the codes.
4. **Never overwrite, never hide.** Output files get ` (1)` suffixes instead
   of being replaced; skipped rows are counted and printed, not silently
   dropped.
5. **Both modes.** Fully scriptable flags (`-f`, `-o`) and a friendly
   interactive Q&A when run by double-click, with `isTerminal` deciding
   whether to wait for Enter at the end.
6. **Small, honest format code.** One `writeX` function per format, each
   tested against a shared sample; amount-signing rules documented per format;
   QIF writes transfers into both accounts so books balance.
7. **Binary hygiene.** `CGO_ENABLED=0`, `-trimpath`, `-ldflags "-s -w"`,
   version stamped from `package.json`; CI job `build-money-extractor` in
   `.github/workflows/build-release.yml` builds and attaches the artifacts.

### Dependency footprint (why extraction was easy)

The extractor's parser depends on only a thin slice of the ledger:

| honey-ledger package | What the parser actually uses | Kept in HB SunRise |
|---|---|---|
| `pkg/core` | `Context` interface, `NewNullContext()` | Yes, verbatim (self-contained) |
| `pkg/errs` | `Error`, `NewNormalError`, `Or`, 5 import error vars | Slim: only those symbols |
| `pkg/log` | `SetQuiet`, `Infof/Warnf/Errorf` | Slim reimplementation on stdlib |
| `pkg/models` | `TransactionType` + 4 constants | Slim |
| `pkg/validators` | `AllCurrencyNames` (ISO 4217 set) | Slim: the map only |
| `pkg/converters/msmoney/msisam` | the engine | Yes, verbatim (stdlib only) |
| external | `abemedia/go-cabinet`, `xuri/excelize/v2` | Same deps |

Everything else the extractor file references (`converter`, `datatable`,
`utils`, the full `models`/`errs`/`log`/`settings` machinery, logrus, xorm) is
only needed by Honey Ledger's *server-side import pipeline* — that code lives
in `microsoft_money_transaction_data_file_importer.go` and
`microsoft_money_transaction_data_table.go`, which the standalone tool does not
use. One small function from the importer file (`IsMicrosoftMoneyDataFile`,
the MSISAM signature check) is needed by the extractor and now lives in
`pkg/converters/msmoney/microsoft_money_data_file_signature.go`.

## Observations and risks

- **The parser is read-only.** `msisam` opens and cracks an existing database;
  there is no writer. Reconstructing a `.mny` file — the core of HB SunRise's
  long-term vision — means implementing the write side: page allocation, the
  scrambled header key block, table definitions, and Money's catalog format,
  guided by `docs/MONEY_FILE_FORMAT.md`. This is the largest piece of work.
- **No committed `.mny` fixture.** The format cannot be redistributed, so the
  engine's real-file tests skip unless `MSISAM_SAMPLE_FILE` points at a file.
  Any SunRise test strategy must keep the same split: synthetic byte-level
  tests committed, real-file tests environment-gated.
- **Licensing.** All copied code is GPL-3.0-or-later with
  `SPDX-FileCopyrightText: 2020-2026 Honey Badger Universe` headers; HB SunRise
  keeps the same license and headers.
- **Currency table is a snapshot.** `validators.AllCurrencyNames` is the ISO
  4217 list as of the source checkout; new codes (e.g. any post-2026 ISO
  amendment) would need a manual update.

## Verdict

Honey Ledger is a well-engineered reference: clean layering, honest
documentation of a reverse-engineered binary format, disciplined error codes,
and a release pipeline that produces static, version-stamped binaries for five
platforms. Its HB Money Extractor is close to the ideal seed for HB SunRise:
the entire read-and-export path extracted with a dependency surface of five
small shim packages and two external libraries.
