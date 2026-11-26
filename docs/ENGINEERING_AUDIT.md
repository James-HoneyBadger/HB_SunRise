# HB SunRise — Engineering Audit

Phase 1 of the HB SunRise workstation program: a complete inventory and review
of the repository as it stands, a trace of the decoding pipeline, confirmed
defects, gaps, risks, and the architectural recommendations the later phases
build on.

Audited revision: `255d106` (main), 2026-11-24. All 6,250 lines of Go, the
tests, the build script, both workflows and the documentation were inspected.
Updated at the end of Phase 2 (see §10) with the defects the phase closed, and
again on 2026-11-26 (see §12) with the defect-closure and hygiene sweep.

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

- **86 tests PASS** in the root module across `msisam`, `msmoney`, `export`
  and the CLI, plus **8 desktop-module tests** (including the end-to-end
  pipeline tests over synthetic `.mny`/`.mbf` files, the malformed-input
  hardening tests, the cross-currency transfer tests, the workspace
  derivation tests, and the concurrent raw-inspection race test).
- **3 tests SKIP** and are therefore **not validated**:
  - `TestRealDatabaseExtract` (`msmoney`)
  - `TestOpenRealDatabase`, `TestRealDatabaseTables` (`msisam`)

  All three require `MSISAM_SAMPLE_FILE` to point at a real Microsoft Money
  file. A real-file validation pass has not been performed in this audit. The
  engine was previously verified against a real Money Sunset database inside
  Honey Ledger (165 accounts, 193 categories, 39,293 transaction rows), but that
  verification is not reproducible here and must not be counted as validation
  of the current code. Re-running it is a standing task (see §8).

No test fails today. `go vet ./...` is clean on both modules; CI runs
`go vet` + `go test -race` on Linux, macOS and Windows, and — since §12 —
a formatting check and the offline check on top.

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
4. ~~Floating point is authoritative in the generic value layer.~~
   **Fixed in §12.** `Database.Value` returns the scaled `int64` the file
   stores for `Money` and `*big.Rat` for `Numeric`; `Float` and `Double`
   remain `float64` because that is their stored IEEE-754 form. `Numeric`
   now decodes the full 16-byte magnitude exactly instead of through
   `uint64` and repeated floating-point division.
5. ~~`Numeric` decoding is lossy by construction.~~ **Fixed in §12.** See
   defect 4: `Database.Numeric` returns `*big.Rat` with `10^scale` as the
   exact denominator; nothing passes through floating point.
6. ~~`expandText` drops a trailing odd byte~~ when a compressed block ends
   with a lone byte in uncompressed state, and a stray `0x00` toggles the
   encoding silently. **Fixed in §12.** A lone trailing byte in the
   uncompressed state is now reported: `String` returns `ErrCorruptRow`,
   a column name that cannot be decoded fails its definition
   (`ErrCorruptDefinition`, counted once), and a catalog name that cannot
   be decoded is counted as an unreadable catalog entry. A stray `0x00`
   toggle that produces even-length but wrong text remains undetectable
   without the original string; it is inherent to the toggle encoding.
7. **Transfer counterpart matching is a heuristic.** Same-day transfers are
   matched by direction, then disambiguated by memo; equal memos in the same
   direction on the same day take the first match. The fallback (source amount
   reused) never fabricates, but the report does not tell the user which
   transfers were matched heuristically versus exactly. Phase 6 requires
   distinguishing recovered from calculated data.
8. ~~`Database` is documented as not thread-safe~~ and memoizes `dataPages` on
   first use. **Fixed in §12 for the desktop app:** the raw inspection
   session is serialized behind a mutex, and a race-detector test walks
   pages concurrently to keep it honest.

## 4. Unsupported record types and missing features

Everything below is *not decoded* today. The program honestly skips investments
and counts them; every other Money table is simply never opened. The Raw
inspection workspace in the desktop app still exposes their rows and columns
for investigation, and unknown fields are never discarded silently.

| Area | Status |
|---|---|
| Investments (purchases/sales, lots, holdings, dividends, fees) | Rows with a security handle are skipped and counted |
| Securities (tickers, CUSIP, prices, exchange rates history) | Not read |
| Budgets (`BGT`), scheduled/recurring transactions, bills | Not read |
| Payees and payee metadata | Name decoded (`PAY` + `TRN.hpay`); addresses and other payee columns not read |
| Reconciliation state and opening balances | Not read |
| Account metadata | Name, currency, closed and hidden flags decoded; type and archive flag not read |
| Exchange rates between currencies in the file | Not read |
| Deep category nesting (3+ levels) | Collapsed to two levels |
| Money 2000 and earlier (2048-byte layout) | Not detected, see defect 2 |
| Password-protected files | Correctly reported as encrypted; no unlock path |

Do not claim support for any of the above until real-file-verified (§8).

## 5. Data-integrity risks

- **Defect 1** (silent decode loss) is the highest-severity item: it can
  silently drop financial records. Closed in Phase 2 (diagnostics), hardened
  later: a corrupt definition is now counted once and the catalog's bare
  definition page is never counted as corruption.
- **Source hash and engine version provenance.** The desktop app records the
  SHA-256 of the input with every extraction; the exporters record the
  exporter name and version. The exporters do not embed the input hash in the
  exported file itself.
- **Skipped rows are listed by reason.** Closed in §12:
  `ExtractedData.Skipped` is now a `SkippedBreakdown` (split totals,
  investment rows, zero-amount rows, rows without an account or date, the
  receiving side of transfers, transfers whose receiving account is unknown);
  the total is unchanged. Both CLIs print the reasons, the desktop overview
  lists them, and the frozen JSON export still carries the bare total.
- **Validation.** Delivered: duplicates, transfer amount fallbacks and
  same-currency mismatches, orphan transfer accounts, accounts without
  transactions, split totals versus fragment sums, broken split references,
  and a calculated summary (counts, totals, date range). Account balances and
  opening/closing balances cannot be validated until those records are
  decoded.
- **Progress and cancellation.** Delivered: `EachRow(ctx, …)` honours
  cancellation and the extraction reports phases; the desktop app has a
  progress bar and a cancel button.

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

**Gate passed (2026-11-24):** the prototype lives in [`desktop/`](../desktop)
— a Wails v2.17 + React 19 + TypeScript application binding the same engine
the CLI uses (`OpenPath`, progress events, cancellation, validation,
diagnostics, native open/save dialogs, recent files, export via the shared
writers, light/dark themes, drag-and-drop). It builds to a single 11.5 MiB
binary on Linux with `wails build -tags webkit2_41` (webkit2gtk-4.1) and runs
cleanly; the Windows/macOS builds use the same Wails toolchain via CI runners.
Wails + React/TypeScript is therefore confirmed as the Phase 3 stack; Fyne
remains the fallback only if a platform turns out to be unpackageable.

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

## 11. Payees, account flags, and split-integrity validation (2026-11-24)

Delivered in both applications:

- `PAY` payees and the `TRN.hpay` column decode; `ExtractedTransaction.Payee`
  feeds the QIF `P` line, the JSON export, Honey Ledger's description
  fallback, and the desktop Payees workspace.
- `ACCT.fClosed` and `ACCT.fHidden` decode into `ExtractedAccount` and are
  shown as badges in the desktop Accounts workspace.
- Validation gained split checks (fragment sums versus the stored parent
  total; broken split references) and a summary of counts, income and expense
  totals, and the date range. The desktop Validation workspace shows the
  summary strip; the CLI `-validate` prints it first.
- Diagnostics no longer count the same corrupt definition once per table
  lookup, and the bare catalog definition page is never counted as corruption.
- The desktop Reports workspace gained a spending-by-payee chart.

Remaining gated on real-file verification (§8): investments, securities,
budgets, scheduled transactions, reconciliation state, opening balances,
exchange rates, and deep category nesting.

## 9. Repository hygiene observed

- Formatting and `go vet`: clean today, but §12 found `gofmt -l` was **not**
  empty on 2026-11-26 (`desktop/app.go`, `validation.go`,
  `validation_test.go`, all touched by `d965104`) while CI had no formatting
  job. Fixed, and CI now fails on any gofmt diff, so the claim cannot rot
  again. The same CI job runs `scripts/check-no-telemetry.sh`, which fails on
  analytics/crash-reporting endpoints and on network calls in the Go and
  frontend code — the offline invariant is machine-enforced now, not just
  asserted.
- Release pipeline verified end-to-end (five binaries, tag-stamped versions,
  `SHA256SUMS.txt`, published at v1.0.0).
- No TODOs, FIXMEs or stray panics in production code (the only `panic`s are in
  the test-only synthetic generator).
- Documentation is current, including the honest "what is left out" section of
  the README; `docs/MONEY_FILE_FORMAT.md` points at Honey Ledger's canonical
  format research document.

## 12. Defect closure and hygiene pass (2026-11-26)

Sweep of §3 and §9 against the working tree, closing every defect that does
not depend on a real Microsoft Money file. Engine changes were applied
identically in Honey Ledger (`validation.go` synced back, defects 4–6 and the
skipped breakdown propagated), keeping the shared engine byte-identical
modulo import paths.

- **Defects 4 and 5 (float64 value layer):** `Database.Value` now returns the
  scaled `int64` for money and `*big.Rat` for numeric; `Database.Numeric`
  returns `*big.Rat` with `10^scale` as the exact denominator. `Float` and
  `Double` stay `float64` — that is their stored IEEE-754 form. The desktop
  raw inspector renders the exact decimal. Tests assert the exact scaled
  value and an exact negative numeric.
- **Defect 6 (silent text truncation):** `expandText` reports a lone trailing
  byte in the uncompressed state as `ErrCorruptRow` instead of dropping it;
  `decodeText` propagates it. A text column with a truncated tail fails its
  row, a truncated column name fails its definition (counted once as a
  corrupt definition), and a truncated catalog name is counted as an
  unreadable catalog entry. Hardening tests cover the new paths.
- **Defect 8 (Database not thread-safe):** the desktop app serializes the raw
  inspection session with a mutex; `TestInspectRowsConcurrentPageWalks`
  walks pages from eight goroutines and fails under `-race` without the lock.
- **Skipped rows listed by reason (§5):** `ExtractedData.Skipped` is a
  `SkippedBreakdown` with one field per reason and `Total()`; both CLIs print
  the reasons and the desktop overview lists them. The JSON export keeps its
  frozen `skippedRows` total. Honey Ledger's importer warning now names the
  reasons too.
- **Hygiene (§9):** the three unformatted files are gofmt-clean again; CI has
  a formatting job and `scripts/check-no-telemetry.sh` (analytics endpoints
  plus network calls in Go and frontend code). The script passes today and
  fails on a planted `net/http` import or `fetch()` call.

Carried forward, unchanged in scope:

- Defect 3 (code page never used) — still needs a non-Western real-file
  reproduction before changing decoding.
- Defect 7 (transfer matching provenance) — Phase 6.
- §8 standing tasks: real-file validation pass and the fixture wish list.
