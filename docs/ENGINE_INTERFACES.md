# The Money Engine Interfaces

How HB SunRise (and, eventually, Honey Ledger through `hb-money-engine`)
consumes the decoding engine. The engine lives in
[`pkg/converters/msmoney`](../pkg/converters/msmoney) with the byte-level
MSISAM reader in [`msisam`](../pkg/converters/msmoney/msisam) and the writers
in [`export`](../pkg/converters/export). The copy in this repository is kept in
lockstep with the copy Honey Ledger carries; never change one without the
other.

Every interface below is real code with callers, except Recovery, which is
specified for Phase 7.

## Recognition

```go
msmoney.IsMicrosoftMoneyDataFile(data []byte) bool // MSISAM header at offset 4
msmoney.IsMicrosoftMoneyBackupFile(data []byte) bool // MSCF cabinet signature
```

A backup is unwrapped in memory and treated exactly like a `.mny`.

## Version detection

`msisam.Open` validates the engine version at header offset `0x14` and returns
`msisam.ErrUnsupportedVersion` for anything that is not the Jet 4 layout
(Money 2000 and earlier). The extraction maps that to
`msmoney.ErrDataFileUnsupportedVersion`, which the CLI translates for the user.
`(*Database).Version()` and `CodePage()` expose the header fields for
diagnostics.

## Database pages, tables, schema, records

```go
database, err := msisam.Open(data)          // page view, header, page count
database.Page(index) []byte                 // raw page
database.PageType(index) byte               // header/data/definition/index/leaf/usage map
database.Definitions() ([]*Definition, error)  // every table definition
database.Catalog() ([]CatalogEntry, error)     // object names and pages
database.Tables() / Table(name)             // named table lookup
database.Definition(page)                   // columns: type, number, offset, size, scale
database.EachRow(ctx, definition, visit)    // decoded fields per live row
database.Value(field) / Int / String / Time // typed accessors
```

The row walk honours `ctx` cancellation and records decoding problems in a
[`Diagnostics`](../pkg/converters/msmoney/msisam/diagnostics.go) sink attached
with `SetDiagnostics`; without a sink it stays silent, as before.

## Financial normalization and extraction

```go
msmoney.ExtractMicrosoftMoneyData(ctx, data) (*ExtractedData, error)
msmoney.ExtractMicrosoftMoneyDataWithOptions(ctx, data, ExtractionOptions{Progress: ...})
```

`ExtractedData` carries:

- accounts with name, currency, and the file's closed and hidden flags,
- transactions (splits flattened with `Split` provenance, transfers matched
  with `TransferAmount`, payees from the `PAY` table, amounts in hundredths as
  integers),
- the skipped-row count, and
- an `ExtractedDiagnostics` report of decoding problems: each corrupt
  definition counts once, rows and definitions never fail silently, and the
  bare catalog definition page is not corruption.

Progress reports phases (`opening`, `reading tables`, `transactions`,
`flattening`) with done/total counts, and cancellation travels through the
context.

## Validation

```go
report := msmoney.ValidateTransactions(data)
report.Confirmed() / report.Uncertain()   // findings by certainty
report.Summary                            // counts, totals, date range
```

Findings carry a severity that distinguishes confirmed facts (exact
duplicates, split totals that do not match their fragments, broken split
references, transfers into accounts the file does not hold) from uncertain
interpretations (a transfer whose receiving side fell back to the source
amount, same-currency transfers whose sides differ). The report never repairs
data: discrepancies are explained, never patched with invented transactions.
`Summary` is a calculation over the transactions the file holds — it is not a
Money account balance, because opening balances are not decoded.

## Export

```go
formats := export.Formats()                // registry: key, name, extension, description
err := export.Write(key, w, source, data) // one entry point for all six formats
```

The CLI's `allFormats` is built from `Formats()`, so the menu and the writers
cannot diverge. Export bytes are frozen compatibility (§AGENTS.md rule 8).
QIF writes the payee on the `P` line and the memo on `M` when both exist;
JSON carries the payee and the account flags; the CSV, TSV, Honey Ledger CSV
and XLSX layouts are unchanged.

## Recovery (Phase 7, specification)

Recovery operates on a partially damaged database and must never write to the
source file. The planned interface:

```go
type Recoverer interface {
    // Probe reports what can be read without changing the file: page count,
    // page type counts, definitions found, and the diagnostics of a read pass.
    Probe(data []byte) (ProbeReport, error)

    // Recover returns every record that can be decoded from the pages that
    // are intact, together with the fragments that could not be reassembled.
    Recover(data []byte) ([]RecoveredRecord, []Fragment, error)
}
```

`Fragment` preserves unknown records and fields verbatim so nothing is
reinterpreted into an ordinary transaction it never was. No implementation
exists yet; do not claim recovery support until real-file-verified.

## Licensing

Engine and consumers are GPL-3.0-or-later with identical
`SPDX-FileCopyrightText: 2020-2026 Honey Badger Universe` headers; sharing the
engine with Honey Ledger is license-compatible. Keep the synchronized-copy
discipline until `hb-money-engine` is published.
