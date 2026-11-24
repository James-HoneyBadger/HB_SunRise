# AGENTS.md — HB SunRise development principles

Permanent rules for anyone (human or agent) working in this repository. These
outrank convenience and must survive refactors. The product brief is the source
of truth for scope; this file is how that brief is implemented in code.

## Product invariants (non-negotiable)

1. **Free and open-source.** GPL-3.0-or-later, same SPDX headers as today.
2. **Windows, macOS and Linux.** Every change must keep `go vet` and the test
   suite green on all three; CI enforces this. Never add a platform-specific
   code path without a CI-visible test.
3. **Completely offline.** No network calls, telemetry, analytics, accounts,
   cloud services, ads, or external financial services. `scripts/check-no-telemetry.sh`
   (in Honey Ledger) and a grep for network APIs are part of the definition of
   done for new code. No AI features are mandatory; any future AI must be
   strictly optional and local.
4. **The original Money file is never modified.** The decoder keeps a read-only
   view; output is written elsewhere. A test that opens a file for writing
   against the input path is a bug.
5. **No silent financial data loss.** Every skipped, rejected, or undecodable
   record must be *counted* and, where possible, *listed with a reason*. See
   ENGINEERING_AUDIT.md defect 1 — closing it is the highest-priority task.
6. **No fabricated financial records.** Unsupported record types (investments,
   budgets, …) are reported as skipped or preserved in a diagnostic form; they
   are never converted into ordinary transactions "to make it balance".
   Validation discrepancies are explained, never repaired with invented
   adjustment entries.
7. **The CLI stays.** The graphical application is the principal product, but
   `cmd/hb-sunrise` keeps working, keeps its flags, exit codes and output
   formats, and keeps passing its tests.
8. **Export compatibility is frozen.** The six formats (csv, tsv, honeyledger,
   xlsx, json, qif) keep their columns, headers, amount-signing rules and
   quoting behavior. Changing an export's bytes is a breaking change and needs
   an explicit decision.

## Financial correctness

9. **Money is integers.** Monetary values flow through the codebase as scaled
   integers (hundredths, thousandths for investment prices, Money's ×10000 raw
   scale where appropriate). Binary floating point must never be the
   authoritative representation. The generic `Value` layer's `float64`
   returns are a known defect (audit §3.4); new code must not rely on them.
10. **Precision is per-domain.** Currency precision, investment quantity
    precision, and exchange-rate precision are preserved independently; never
    round one domain through another's scale.
11. **Distinguish data from interpretation.** Recovered original fields,
    heuristically matched relationships (e.g. transfer counterpart matching)
    and calculated aggregates must be distinguishable in the data model, the
    validation report, and the UI.

## Architecture

12. **The engine has no UI, no files, no network.** Packages under
    `pkg/converters/msmoney/` take `[]byte` and return plain structs. The GUI
    and CLI are clients of the same interfaces. Progress and cancellation
    belong in the interfaces (Phase 2), never in ad-hoc callbacks.
13. **One decoding implementation.** This repository's copy of the engine is
    kept byte-identical to Honey Ledger's copy (synchronized from the private
    canonical source). Never patch a decoding behavior here without
    propagating the identical change there; divergent behavior between the two
    applications is a bug.
14. **Graphical framework:** Wails + React/TypeScript is the working plan,
    Fyne is the fallback. Do not commit to either until the packaging spike
    (audit §7.1) proves Linux/macOS/Windows packaging. Users must never need a
    separate runtime or a terminal.
15. **New decoders are feature-gated by verification.** A record type is not
    "supported" until it is verified against a real Microsoft Money file.
    Until then it is reported, never silently interpreted.

## Testing

16. **Skipped tests are not validation.** The three real-file tests skip
    without `MSISAM_SAMPLE_FILE`. Reports, releases and reviews must say
    "64 passed, 3 skipped" — never "all passed". A real-file validation pass is
    a standing task (audit §8).
17. **Tests are required for behavior change.** Every fix or feature ships with
    a focused test that fails without it. Synthetic fixtures are preferred;
    byte-level fixtures live in the test files, whole-file fixtures come from
    `pkg/converters/msmoney/synthetic`.
18. **Hardening tests for parsers.** Anything that reads attacker-controllable
    bytes gets bounds, clamp, and malformed-input tests (see the existing
    `TestOpenNeverPanicsOnMalformedInput` pattern). No parser code merges
    without them.
19. **Commands before claiming done.** The definition of done for any change:
    `gofmt -l` empty, `go vet ./...` clean, `go test -count=1 -race ./...`
    green, and — when engines or exports change — the equivalent suite in
    Honey Ledger green too.

## Release discipline

20. **The git tag is the only version source.** No version strings in source;
    `scripts/build-hb-sunrise.sh` stamps `main.version` from
    `git describe --tags --exact-match`. Never commit a bumped version file.
21. **Never overwrite output.** `uniquePath` semantics stay: a numbered
    variant is created, an existing export is never replaced.
22. **Never hide skipped rows.** User-facing output always reports the skipped
    count, and the GUI must show skipped and failed records explicitly.

## Documentation

23. **Format research is a first-class artifact.** `docs/MONEY_FILE_FORMAT.md`
    and the canonical format document in Honey Ledger are updated whenever a
    decoding behavior changes or a new field is interpreted. Offsets and
    semantics must stay accurate — the next person reverse-engineering Money
    depends on them.
24. **Audit trail.** ENGINEERING_AUDIT.md is re-dated and updated at the end of
    each phase; defects are numbered and either fixed (with the fixing commit)
    or carried forward with a reason.

## Code style

25. **Minimal comments.** Comment only what needs explanation: format offsets,
    non-obvious invariants, why-not-what. Follow the existing SPDX header on
    every new file.
26. **No dependencies without cause.** The engine is stdlib plus go-cabinet;
    the writers add excelize. New dependencies need a written justification in
    the commit message.
