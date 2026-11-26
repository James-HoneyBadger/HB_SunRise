// SPDX-License-Identifier: GPL-3.0-or-later
import type { TransactionView } from './bindings'

export interface LedgerFilter {
  query: string
  account: string
  type: string
  currency: string
  from: string
  through: string
}
export const emptyLedgerFilter: LedgerFilter = { query: '', account: '', type: '', currency: '', from: '', through: '' }

// Filter decoded source rows; transfers are included once, even when the
// selected account is their receiving side. Currency refers to the source row.
export function filterLedger(rows: TransactionView[], f: LedgerFilter): TransactionView[] {
  const terms = f.query.trim().toLowerCase().split(/\s+/).filter(Boolean)
  return rows.filter(t => {
    if (f.account && t.account !== f.account && !(t.type === 'Transfer' && t.transferAccount === f.account)) return false
    if (f.type && t.type !== f.type) return false
    if (f.currency && t.currency !== f.currency) return false
    if (f.from && t.time < f.from || f.through && t.time > f.through) return false
    const hay = [t.account, t.payee, t.currency, t.memo, t.category, t.subCategory, t.transferAccount, t.type, t.time].join(' ').toLowerCase()
    return terms.every(term => hay.includes(term))
  })
}

export interface LedgerTotal {
  currency: string
  income: bigint
  expense: bigint
  valid: boolean
}

// Never combine different currencies or count transfers as cash flow.
// BigInt preserves exact minor units during aggregation; reject source values
// that have already exceeded the safe integer range of the desktop bridge.
export function summarizeLedger(rows: TransactionView[]): LedgerTotal[] {
  const totals = new Map<string, LedgerTotal>()
  for (const t of rows) {
    if (t.type !== 'Income' && t.type !== 'Expense') continue
    let total = totals.get(t.currency)
    if (!total) {
      total = { currency: t.currency, income: 0n, expense: 0n, valid: Boolean(t.currency) }
      totals.set(t.currency, total)
    }
    if (!Number.isSafeInteger(t.amount)) { total.valid = false; continue }
    if (t.type === 'Income') total.income += BigInt(t.amount)
    else total.expense += BigInt(t.amount)
  }
  return [...totals.values()].sort((a,b) => a.currency.localeCompare(b.currency))
}

export function formatMinor(value: bigint): string {
  const absolute = value < 0n ? -value : value
  return (value < 0n ? '-' : '') + (absolute / 100n).toString() + '.' + (absolute % 100n).toString().padStart(2, '0')
}
