// SPDX-License-Identifier: GPL-3.0-or-later
import type { TransactionView } from './bindings'

export interface HistoryPoint { date: string; value: number }
export interface HistoryResult { points: HistoryPoint[]; issue: string }

// Only chart complete, safely representable changes. Never substitute an
// outgoing amount for an unknown incoming amount. Opening balances are absent.
export function accountHistory(rows: TransactionView[], account: string): HistoryResult {
  const ordered = rows.filter(t => !account || t.account === account || (t.type === 'Transfer' && t.transferAccount === account))
    .sort((a,b) => a.time.localeCompare(b.time))
  const points: HistoryPoint[] = []
  let running = 0n
  const fail = (issue: string): HistoryResult => ({ points: [], issue })
  for (const t of ordered) {
    const outgoing = !account || t.account === account
    const incoming = !account || t.transferAccount === account
    if (t.type === 'Transfer') {
      if (outgoing) {
        if (!Number.isSafeInteger(t.amount)) return fail('Unsafe source amount.')
        running -= BigInt(t.amount)
      }
      if (incoming) {
        if (!Number.isSafeInteger(t.transferAmount) || t.transferAmount === 0) return fail('An incoming transfer has an unknown or unsafe receiving amount.')
        running += BigInt(t.transferAmount)
      }
    } else if (t.type === 'Income' || t.type === 'Expense') {
      if (!Number.isSafeInteger(t.amount)) return fail('Unsafe source amount.')
      running += BigInt(t.amount) * (t.type === 'Income' ? 1n : -1n)
    } else return fail('An unsupported transaction type prevents complete history.')
    if (running > BigInt(Number.MAX_SAFE_INTEGER) || running < BigInt(Number.MIN_SAFE_INTEGER)) return fail('Cumulative change exceeds safe chart precision.')
    points.push({ date: t.time, value: Number(running) })
  }
  return { points, issue: '' }
}
