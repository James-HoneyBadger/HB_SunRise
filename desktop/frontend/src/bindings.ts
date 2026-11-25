// Shapes of the data the Go backend returns. The runtime functions themselves
// live in the generated wailsjs modules next to this file.

export interface Progress {
  Phase: string
  Done: number
  Total: number
}

export interface FileInfo {
  path: string
  name: string
  size: number
  sha256: string
}

export interface AccountView {
  name: string
  currency: string
}

export interface TransactionView {
  time: string
  type: string
  account: string
  currency: string
  amount: number
  transferAccount: string
  transferAmount: number
  category: string
  subCategory: string
  memo: string
}

export interface FindingView {
  severity: string
  kind: string
  account: string
  transferAccount: string
  time: string
  amount: number
  message: string
}

export interface DiagnosticsView {
  CorruptRows: number
  UnresolvedRows: number
  CorruptDefinitions: number
  UnknownCatalogEntries: number
  Samples: string[]
}

export interface ExtractionView {
  file: FileInfo
  accounts: AccountView[]
  transactions: TransactionView[]
  skipped: number
  diagnostics: DiagnosticsView
  validation: FindingView[]
  elapsed: string
}
