// SVG chart primitives and report aggregations. Charts are drawn by hand so
// the application stays dependency-light and fully offline.

import { TransactionView } from './bindings'

export interface MonthlyPoint {
  month: string
  income: number
  expense: number
}

export interface CategorySlice {
  name: string
  total: number
}

export interface AccountPoint {
  date: string
  value: number
}

function monthOf(date: string): string {
  return date.slice(0, 7)
}

// monthlySeries buckets income and expense into consecutive months.
export function monthlySeries(transactions: TransactionView[]): MonthlyPoint[] {
  const byMonth = new Map<string, MonthlyPoint>()

  for (const transaction of transactions) {
    const month = monthOf(transaction.time)
    let point = byMonth.get(month)

    if (!point) {
      point = { month, income: 0, expense: 0 }
      byMonth.set(month, point)
    }

    if (transaction.type === 'Income') {
      point.income += transaction.amount
    } else if (transaction.type === 'Expense') {
      point.expense += transaction.amount
    }
  }

  return [...byMonth.values()].sort((left, right) => left.month.localeCompare(right.month))
}

// spendingByCategory sums expenses by their full category path, largest first.
export function spendingByCategory(transactions: TransactionView[], limit: number): CategorySlice[] {
  const totals = new Map<string, number>()

  for (const transaction of transactions) {
    if (transaction.type !== 'Expense') continue

    const path = [transaction.category, transaction.subCategory].filter(Boolean).join(' / ') || '(uncategorized)'
    totals.set(path, (totals.get(path) ?? 0) + transaction.amount)
  }

  return [...totals.entries()]
    .map(([name, total]) => ({ name, total }))
    .sort((left, right) => right.total - left.total)
    .slice(0, limit)
}

// spendingByPayee sums expenses by the payee the file names, largest first.
export function spendingByPayee(transactions: TransactionView[], limit: number): CategorySlice[] {
  const totals = new Map<string, number>()

  for (const transaction of transactions) {
    if (transaction.type !== 'Expense' || !transaction.payee) continue

    totals.set(transaction.payee, (totals.get(transaction.payee) ?? 0) + transaction.amount)
  }

  return [...totals.entries()]
    .map(([name, total]) => ({ name, total }))
    .sort((left, right) => right.total - left.total)
    .slice(0, limit)
}

// cumulativeByAccount computes the running balance change each account's
// transactions in the file imply. It is a calculation over the decoded
// transactions, not a Money account balance: the file's opening balances are
// not decoded, so the report labels the chart accordingly.
export function cumulativeByAccount(transactions: TransactionView[], account: string): AccountPoint[] {
  const ordered = transactions.filter((t) => account === '' || t.account === account || (t.type === 'Transfer' && t.transferAccount === account))
  ordered.sort((left, right) => left.time.localeCompare(right.time))

  let running = 0
  const points: AccountPoint[] = []

  for (const transaction of ordered) {
    if (transaction.type === 'Income') running += transaction.amount
    else if (transaction.type === 'Expense') running -= transaction.amount
    else if (transaction.type === 'Transfer') {
      if (account === '' || transaction.account === account) running -= transaction.amount
      if (account === '' || transaction.transferAccount === account) {
        running += transaction.transferAmount > 0 ? transaction.transferAmount : transaction.amount
      }
    }
    points.push({ date: transaction.time, value: running })
  }

  return points
}

export function amountLabel(amount: number): string {
  return (amount / 100).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

interface BarChartProps {
  months: MonthlyPoint[]
}

// IncomeVersusExpense draws one grouped bar per month.
export function IncomeVersusExpense({ months }: BarChartProps) {
  const width = 720
  const height = 260
  const pad = 44
  const max = Math.max(1, ...months.flatMap((point) => [point.income, point.expense]))
  const group = (width - pad * 2) / Math.max(1, months.length)
  const bar = Math.min(16, (group * 0.35) / 2)

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="chart" role="img" aria-label="Income versus expenses by month">
      {months.map((point, index) => {
        const x = pad + index * group + group / 2
        const incomeHeight = (point.income / max) * (height - pad - 14)
        const expenseHeight = (point.expense / max) * (height - pad - 14)

        return (
          <g key={point.month}>
            <rect x={x - bar - 1.5} y={height - pad - incomeHeight} width={bar} height={incomeHeight} fill="var(--income)" />
            <rect x={x + 1.5} y={height - pad - expenseHeight} width={bar} height={expenseHeight} fill="var(--expense)" />
            {months.length <= 24 && (
              <text x={x} y={height - pad + 13} textAnchor="middle" className="axis">
                {point.month}
              </text>
            )}
          </g>
        )
      })}
      <line x1={pad - 8} y1={height - pad} x2={width - pad} y2={height - pad} stroke="var(--line)" />
      <text x={pad - 8} y={height - pad + 14} className="axis">
        0
      </text>
    </svg>
  )
}

interface HBarChartProps {
  slices: CategorySlice[]
  title?: string
}

// CategoryBars draws the largest spending slices as horizontal bars.
export function CategoryBars({ slices, title = 'Spending by category' }: HBarChartProps) {
  const width = 720
  const row = 30
  const label = 220
  const height = slices.length * row + 18
  const max = Math.max(1, ...slices.map((slice) => slice.total))

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="chart" role="img" aria-label={title}>
      {slices.map((slice, index) => {
        const y = index * row + 10
        const length = (slice.total / max) * (width - label - 90)

        return (
          <g key={slice.name}>
            <text x={0} y={y + 15} className="axis" textAnchor="start">
              {slice.name.length > 34 ? slice.name.slice(0, 33) + '…' : slice.name}
            </text>
            <rect x={label} y={y} width={length} height={18} rx={3} fill="var(--accent)" />
            <text x={label + length + 8} y={y + 14} className="axis">
              {amountLabel(slice.total)}
            </text>
          </g>
        )
      })}
    </svg>
  )
}

interface LineChartProps {
  points: AccountPoint[]
}

// CumulativeLine draws the running change implied by the transactions in the
// file. It is explicitly a calculation, never presented as a Money balance.
export function CumulativeLine({ points }: LineChartProps) {
  const width = 720
  const height = 260
  const pad = 44

  if (points.length < 2) {
    return <p className="muted">Not enough transactions to draw a history.</p>
  }

  const values = points.map((point) => point.value)
  const min = Math.min(0, ...values)
  const max = Math.max(0, ...values)
  const span = Math.max(1, max - min)

  const x = (index: number) => pad + (index / (points.length - 1)) * (width - pad * 2)
  const y = (value: number) => height - pad - ((value - min) / span) * (height - pad * 2)

  const path = points.map((point, index) => `${index === 0 ? 'M' : 'L'}${x(index).toFixed(1)},${y(point.value).toFixed(1)}`).join(' ')

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="chart" role="img" aria-label="Cumulative change from transactions">
      <path d={path} fill="none" stroke="var(--accent)" strokeWidth={2} />
      <line x1={pad - 8} y1={height - pad} x2={width - pad} y2={height - pad} stroke="var(--line)" />
      <text x={pad - 8} y={height - pad + 14} className="axis">
        {amountLabel(0)}
      </text>
      <text x={pad - 8} y={16} className="axis">
        {amountLabel(max)}
      </text>
      <text x={pad - 8} y={height - pad - 6} className="axis">
        {amountLabel(min)}
      </text>
      <text x={pad - 8} y={height - pad + 30} className="axis">
        {points[0].date}
      </text>
      <text x={width - pad} y={height - pad + 30} className="axis" textAnchor="end">
        {points[points.length - 1].date}
      </text>
    </svg>
  )
}
