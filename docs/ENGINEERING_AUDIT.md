# HB SunRise — Engineering Audit

Phase 1 of the HB SunRise workstation program: a complete inventory and review
of the repository as it stands, a trace of the decoding pipeline, confirmed
defects, gaps, risks, and the architectural recommendations the later phases
build on.

Audited revision: `255d106` (main), 2026-11-24. All 6,250 lines of Go, the
tests, the build script, both workflows and the documentation were inspected.
Updated at the end of Phase 2 (see §10) with the defects the phase closed.

---

## 1. What exists today

HB SunRise is a single-binary command line extractor that reads Microsoft Money
`.mny` data files and `.mbf` backups and writes six export formats. It carries
its own copy of the decoding engine (kept in lockstep with the copy Honey Ledger
carries), has no GUI, and is fully offline.

| Area | Files | Lines |
|---|---|---|
| CLI | `cmd/hb-sunrise/main.go`, `formats.go` (+ 3 test files) | ~1,850 |
| Extraction core | `pkg/converters/msmoney/*.go` (reader, extract, backup, signature, errors, log, context, currencies) | ~1,700 |
| MSISAM engine | `pkg/converters/msmoney/msisam/{catalog,database,definition,row,value}.go` | ~1,565 |
| Export writers | `pkg/converters/export/{export.go,export_test.go}` | ~630 |
| Synthetic test files | `pkg/converters/msmoney/synthetic/synthetic.go` | ~530 |
| Support | `pkg/core`, `pkg/log` | ~100 |
| Build/release | `scripts/build-hb-sunrise.sh`, `.github/workflows/{ci,release}.yml` | — |
| Docs | `README.md`, `docs/{BUILDING_HB_SUNRISE,HONEY_LEDGER_REVIEW,MONEY_FILE_FORMAT}.md` | — |

### Test status (run fresh, `go test -count=1 ./...`)

- **64 tests PASS** across `msisam`, `msmoney`, `export` and the CLI (including
  the end-to-end pipeline tests over synthetic `.mny`/`.mbf` files, the
  malformed-input hardening tests, and the cross-currency transfer tests).
- **3 tests SKIP** and are therefore **not validated**:
  - `TestRealDatabaseExtract` (`msmoney`)
  - `TestOpenRealDatabase`, `TestRealDatabaseTables` (`msisam`)

  All three require `MSISAM_SAMPLE_FILE` to point at a real Microsoft Money
  file. A real-file validation pass has not been performed in this audit. The
  engine was previously verified against a real Money Sunset database inside
  Honey Ledger (165 accounts, 193 categories, 39,293 transaction rows), but that
  verification is not reproducible here and must not be counted as validation
  of the current code. Re-running it is a standing task (see §8).

No test fails today. `go vet ./...` and `gofmt` are clean; CI runs
`go vet` + `go test -race` on Linux, macOS and Windows and is green.

---

## 2. Decoding pipeline trace

Raw bytes to exported files, component by component:

```
.mny / .mbf
  │  cmd/hb-sunrise/main.go        read file (never writes to it)
  ▼
microsoft_money_backup_archive.go  .mbf? unpack cabinet in memory
  │                                 (≤32 members, ≤512 MiB, header probe)
  ▼
microsoft_money_data_file_signature.go   "MSISAM Database" at offset 4?
  ▼
msisam.Database.Open
  ├─ magic check → ErrNotMSISAMDatabase
  ├─ page size: whole 4096-byte pages only (trailing data ignored)
  ├─ RC4(headerKey) unscrambles bytes 0x18..0x97 (code page, key, language)
  └─ database key ≠ 0 → ErrEncryptedDatabase (password-protected)
  ▼
catalog.go        Tables()  = definitions + catalog names ("MSysObjects")
                    └─ unnamed tables derive names from identity column "h*"
  ▼
definition.go     Definition: 25-byte column entries over a chain of
                    definition pages (≤64 continuations), column names,
                    types, fixed offsets, null-mask bit, precision/scale
  ▼
row.go            EachRow: page → row slot table (rows grow down from the
                    page end; deleted rows skipped; relocated rows followed)
                    → rowLayoutAt (column count, null mask, variable offsets)
                    → crackRow → []Field (raw bytes, aliasing the buffer)
  ▼
value.go          Value/Int/money/String/Time/GUID/Bytes
                    text: compressed UTF-16 toggle encoding → UTF-16 → string
                    memo: inline / single leaf page / chained leaf pages
                    datetime: IEEE-754 day count from 1899-12-30
                    money: scaled int64 (×10000)
  ▼
microsoft_money_data_reader.go
  ├─ CRNC → ISO 4217 codes (validated against the ISO table)
  ├─ ACCT → accounts {name, currency}
  ├─ CAT  → categories, two-level grouping under INCOME/EXPENSE roots
  ├─ TRN  → transaction rows {handle, account, link, category, security,
  │         amount, memo, date}
  └─ TRN_SPLIT → parent/child handles
  ▼
flattenMicrosoftMoneyTransactions
  ├─ drops split totals (parents), keeps one transaction per split
  ├─ drops investment rows (security handle set) — counted, never converted
  ├─ transfers: only the outgoing row becomes a transaction; the receiving
  │   side's amount is matched same-day (memo-disambiguated) and preserved
  │   per currency; falls back to the source amount when ambiguous
  ├─ amounts: scaled ×10000 → hundredths, round half away from zero
  └─ Skipped counts every row that cannot become a transaction
  ▼
microsoft_money_extract.go        ExtractedData (plain structs, string types)
  ▼
export package                     CSV, TSV, Honey Ledger CSV, XLSX, JSON, QIF
```

The CLI owns I/O, interactivity, error translation and exit codes; the engine
never touches the file system and never modifies the input buffer.

---

## 3. Confirmed defects

1. ~~Silent row loss on decode failure.~~ **Fixed in Phase 2.** `msisam`
   gained a `Diagnostics` sink (`SetDiagnostics`) recording corrupt and
   unresolvable rows, corrupt definitions and unreadable catalog entries;
   `EachRow` skips only rows actually marked deleted. The extraction surfaces
   the report as `ExtractedData.Diagnostics`, both CLIs print it, and Honey
   Ledger's importer logs it. A damaged file can no longer lose records
   silently.
2. ~~Engine version is not enforced.~~ **Fixed in Phase 2.** `Open` returns
   `msisam.ErrUnsupportedVersion` for any engine version other than the Jet 4
   layout; the extraction maps it to `ErrDataFileUnsupportedVersion` and both
   CLIs explain it to the user. A Money 2000-era file is still not read, but
   it is now reported as what it is.
3. **Code-page is decoded but never used.** `CodePage()` reads the Windows code
   page, yet all text is decoded as UTF-16. Non-ASCII text that Money stores
   single-byte encoded under a non-Western code page will decode incorrectly.
   (Needs a real-file reproduction before changing; today only ASCII is
   verified.)
4. **Floating point is authoritative in the generic value layer.**
   `Database.Value` returns `float64` for `Money`, `Float`, `Double` and
   `Numeric`. The extraction path avoids this by reading raw scaled integers,
   so exported money is exact — but any future consumer of `Value` (a database
   inspector, a validator) would get binary floating point for money. Phase 6
   requires exact arithmetic: the value layer must expose scaled integers (or
   `big.Rat`) as the primary form, with float only as an opt-in convenience.
5. **`Numeric` decoding is lossy by construction.** The 16-byte magnitude is
   crammed into `uint64` then scaled by repeated floating-point division;
   magnitudes wider than 64 bits and non-decimal scales lose precision.
   Nothing in the current pipeline uses `Numeric`; it is nevertheless a latent
   correctness trap for Phase 5 (shares, prices).
6. **`expandText` drops a trailing odd byte** when a compressed block ends with
   a lone byte in uncompressed state, and a stray `0x00` toggles the encoding
   silently. Text with embedded NULs or odd tail bytes can decode short with no
   error. Low probability, unverified against real files.
7. **Transfer counterpart matching is a heuristic.** Same-day transfers are
   matched by direction, then disambiguated by memo; equal memos in the same
   direction on the same day take the first match. The fallback (source amount
   reused) never fabricates, but the report does not tell the user which
   transfers were matched heuristically versus exactly. Phase 6 requires
   distinguishing recovered from calculated data.
8. **`Database` is documented as not thread-safe** and memoizes `dataPages` on
   first use. Fine for today's single-threaded extractor; the GUI (background
   extraction with cancellation) must either serialize or snapshot.

## 4. Unsupported record types and missing features

Everything below is *not decoded* today. The program honestly skips investments
and counts them; every other Money table is simply never opened.

| Area | Status |
|---|---|
| Investments (purchases/sales, lots, holdings, dividends, fees) | Rows with a security handle are skipped and counted |
| Securities (tickers, CUSIP, prices, exchange rates history) | Not read |
| Budgets (`BGT`), scheduled/recurring transactions, bills | Not read |
| Payees and payee metadata | Not read |
| Reconciliation state and opening balances | Not read |
| Account metadata (type, closed/archived, hidden) | Only name + currency |
| Exchange rates between currencies in the file | Not read |
| Deep category nesting (3+ levels) | Collapsed to two levels |
| Money 2000 and earlier (2048-byte layout) | Not detected, see defect 2 |
| Password-protected files | Correctly reported as encrypted; no unlock path |

Do not claim support for any of the above until real-file-verified (§8).

## 5. Data-integrity risks

- **Defect 1** (silent decode loss) is the highest-severity item: it can
  silently drop financial records.
- **No source hash or engine version is recorded in outputs.** JSON records the
  exporter name and version but not the SHA-256 of the input or the decoder
  version. Phase 7 requires both for provenance.
- **Skipped rows are only a count.** There is no list of *what* was skipped and
  why (investment vs. split total vs. missing amount/account/date). A
  validation report cannot be built from the current `Skipped int`.
- **No validation pass.** Nothing cross-checks account balances, split totals,
  transfer symmetry, date ranges, duplicates, orphans, or that the number of
  decoded rows equals the table's declared row count. Phase 6 is entirely
  unbuilt.
- **No progress or cancellation.** Extraction is synchronous and atomic. For
  large files the GUI would freeze; a 40,000-row file takes seconds today but a
  future deep decoder must report progress.

## 6. What is strong and must be preserved

- Integer money through the entire extraction pipeline; no fabricated
  transactions; conservative transfer heuristics with a documented fallback.
- Input is never written: `Open` keeps a private copy of the unscrambled header
  only; the original `[]byte` is never mutated.
- Hardened parsing: every length, count and page reference is bounds-checked;
  allocation clamps exist for row counts and chained long values; a
  mutation-fuzz test (`TestOpenNeverPanicsOnMalformedInput`) covers the header.
- Honest error model: five stable sentinel errors mapped to user-facing
  messages; encrypted files are reported as encrypted, not parsed into garbage.
- Deterministic, self-contained releases: five static binaries, tag-stamped
  versions, checksums, CI on three OSes.
- The synthetic `.mny`/`.mbf` generator makes the whole pipeline testable
  without a redistributable Money file, including an end-to-end CLI test.

## 7. Architecture recommendations

### 7.1 Native graphical infrastructure (Phase 3)

Evaluated against the constraints (free, offline, Win/mac/Linux, no separate
runtime for end users, data-dense professional UI):

| Option | Verdict |
|---|---|
| **Wails v2 + React/TypeScript** | **Recommended.** Go stays the engine and backend of one process; the UI is a normal web frontend rendered in the OS webview (WebView2 / WKWebView / WebKitGTK) — the same architecture Honey Ledger already proves in production with Electron, but lighter and without bundling a browser. React + TS gives mature data-grid, charting and theming ecosystems for Phases 4 and 8, and Wails ships native Windows/macOS/Linux binaries that need no installed runtime. |
| Fyne | Workable fallback. Pure Go, no webview, already in use for Honey Ledger's admin tool. But professional virtualized data grids, rich charts and accessibility work would be built largely by hand. |
| Gio | Not recommended for this product. Immediate-mode, excellent for custom drawing, poor fit for a data-heavy workstation UI at this team's size. |

Decision gate: **prototype desktop packaging before committing** — build a
Wails "open file → progress → table" spike, package it for all three OSes and
verify the Linux WebKitGTK story (the one real packaging risk) before Phase 3
proceeds. If Linux packaging proves brittle, fall back to Fyne.

### 7.2 Engine separation (Phase 2) — delivered

The interfaces now exist with callers, and are documented in
[docs/ENGINE_INTERFACES.md](ENGINE_INTERFACES.md):

- **Recognition** — `IsMicrosoftMoneyDataFile` / `IsMicrosoftMoneyBackupFile`.
- **Version detection** — enforced in `Open` (defect 2 closed).
- **Decoding** — pages, tables, schema and records, with a `Diagnostics` sink
  instead of silent skips (defect 1 closed) and `EachRow(ctx, …)` cancellation.
- **Normalization** — `ExtractMicrosoftMoneyDataWithOptions` with per-phase
  progress and `ExtractedDiagnostics` in the result.
- **Validation** — `ValidateTransactions` with confirmed/uncertain findings
  (duplicates, transfer fallbacks, inactive accounts); the report repairs
  nothing.
- **Export** — the `export.Formats()` registry, which both CLIs build their
  menus from.
- **Recovery** — specified, not implemented (Phase 7).

The engine remains byte-identical to Honey Ledger's copy; both applications
were synchronized and both suites pass. Licensing is compatible (GPL-3.0-or-
later, identical headers).

### 7.3 Sequencing

1. ~~Fix defects 1 and 2~~ — done in Phase 2.
2. Build the full `Validator` surface (balances, split totals, symmetry, date
   ranges, orphans) on the existing report.
3. Wails packaging spike, then the Phase 3 GUI skeleton on the interfaces.
4. Deep decoding (Phase 5) table by table, each behind real-file verification.

## 8. Standing tasks

- **Re-run the real-file validation suite** with `MSISAM_SAMPLE_FILE` set to a
  genuine Microsoft Money file and record the results in this document (file
  metadata, counts, any new findings). Until then the three skipped tests are
  explicitly *not validated*.
- Obtain or synthesize fixtures that exercise: non-Western code pages, deep
  category nesting, chained memos beyond one leaf page, relocated and deleted
  rows, empty tables, and a password-protected file.
- After defect 2 lands, obtain a Money 2000-era file to confirm the version
  gate reports it cleanly.

## 10. Phase 2 update (2026-11-24)

Defects 1 and 2 are closed, as noted in §3. The engine interfaces are
delivered and documented in [docs/ENGINE_INTERFACES.md](ENGINE_INTERFACES.md);
both applications carry the synchronized engine and are green. Test counts
after Phase 2: see §1; the three real-file skips remain, unchanged.

## 9. Repository hygiene observed

- Formatting (`gofmt`) and `go vet` clean; CI green on three OSes.
- Release pipeline verified end-to-end (five binaries, tag-stamped versions,
  `SHA256SUMS.txt`, published at v1.0.0).
- No TODOs, FIXMEs or stray panics in production code (the only `panic`s are in
  the test-only synthetic generator).
- Documentation is current, including the honest "what is left out" section of
  the README; `docs/MONEY_FILE_FORMAT.md` points at Honey Ledger's canonical
  format research document.
