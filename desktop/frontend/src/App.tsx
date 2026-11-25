import { useCallback, useEffect, useMemo, useState } from 'react'
import { EventsOn } from '../wailsjs/runtime/runtime'
import {
  OpenFileDialog,
  OpenPath,
  Cancel,
  SaveFileDialog,
  Export,
  RecentFiles,
  ClearRecentFiles,
} from '../wailsjs/go/main/App'
import type { ExtractionView, Progress } from './bindings'
import { useVirtual } from './virtual'
import { IncomeVersusExpense, CategoryBars, CumulativeLine, monthlySeries, spendingByCategory, cumulativeByAccount } from './charts'

type Tab = 'overview' | 'accounts' | 'transactions' | 'reports' | 'validation' | 'raw'

const FORMATS: { key: string; name: string }[] = [
  { key: 'csv', name: 'CSV' },
  { key: 'honeyledger', name: 'Honey Ledger CSV' },
  { key: 'tsv', name: 'TSV' },
  { key: 'xlsx', name: 'Excel workbook' },
  { key: 'json', name: 'JSON' },
  { key: 'qif', name: 'QIF' },
]

function amountOf(amount: number): string {
  return (amount / 100).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function humanSize(size: number): string {
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KiB`
  return `${(size / 1024 / 1024).toFixed(1)} MiB`
}

export default function App() {
  const [view, setView] = useState<ExtractionView | null>(null)
  const [progress, setProgress] = useState<Progress | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [tab, setTab] = useState<Tab>('overview')
  const [theme, setTheme] = useState<'light' | 'dark'>(
    window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light',
  )
  const [recent, setRecent] = useState<string[]>([])
  const [exportKey, setExportKey] = useState('honeyledger')
  const [exportNote, setExportNote] = useState('')
  const [query, setQuery] = useState('')
  const [sort, setSort] = useState<{ field: 'time' | 'amount' | 'account'; dir: 1 | -1 }>({ field: 'time', dir: 1 })

  const refreshRecent = useCallback(() => {
    RecentFiles().then(setRecent).catch(() => setRecent([]))
  }, [])

  useEffect(() => {
    refreshRecent()
    const cancel = EventsOn('extract-progress', (data: unknown) => {
      setProgress(data as Progress)
    })
    return cancel
  }, [refreshRecent])

  useEffect(() => {
    document.documentElement.dataset.theme = theme
  }, [theme])

  const open = useCallback(
    async (path: string) => {
      if (!path || busy) return
      setBusy(true)
      setError('')
      setExportNote('')
      setProgress({ Phase: 'opening', Done: 0, Total: 0 })
      setView(null)
      try {
        const result = await OpenPath(path)
        setView(result)
        setTab('overview')
        refreshRecent()
      } catch (err) {
        setError(String(err))
      } finally {
        setBusy(false)
        setProgress(null)
      }
    },
    [busy, refreshRecent],
  )

  const openDialog = useCallback(async () => {
    const path = await OpenFileDialog()
    if (path) await open(path)
  }, [open])

  const onDrop = useCallback(
    (event: React.DragEvent) => {
      event.preventDefault()
      const file = event.dataTransfer.files[0] as (File & { path?: string }) | undefined
      const path = file?.path
      if (path) void open(path)
    },
    [open],
  )

  const exportTo = useCallback(async () => {
    if (!view) return
    setExportNote('')
    try {
      const path = await SaveFileDialog(exportKey)
      if (!path) return
      await Export(path, exportKey)
      setExportNote(`Wrote ${path}`)
    } catch (err) {
      setExportNote(`Export failed: ${err}`)
    }
  }, [view, exportKey])

  const clearRecent = useCallback(async () => {
    await ClearRecentFiles()
    refreshRecent()
  }, [refreshRecent])

  const transactions = useMemo(() => {
    if (!view) return []
    const filtered = view.transactions.filter((t) => {
      const hay = `${t.account} ${t.memo} ${t.category} ${t.subCategory} ${t.transferAccount} ${t.type}`.toLowerCase()
      return hay.includes(query.toLowerCase())
    })
    const dir = sort.dir
    return [...filtered].sort((a, b) => {
      if (sort.field === 'time') return a.time.localeCompare(b.time) * dir
      if (sort.field === 'amount') return (a.amount - b.amount) * dir
      return a.account.localeCompare(b.account) * dir
    })
  }, [view, query, sort])

  const [reportAccount, setReportAccount] = useState('')

  const confirmed = view?.validation.filter((f) => f.severity === 'confirmed').length ?? 0
  const uncertain = view?.validation.filter((f) => f.severity === 'uncertain').length ?? 0

  const virtual = useVirtual(transactions.length, 34)

  const monthly = useMemo(() => (view ? monthlySeries(view.transactions) : []), [view])
  const categories = useMemo(() => (view ? spendingByCategory(view.transactions, 10) : []), [view])
  const cumulative = useMemo(
    () => (view ? cumulativeByAccount(view.transactions, reportAccount) : []),
    [view, reportAccount],
  )

  const diagnosticsEmpty =
    view === null || (view.diagnostics.CorruptRows === 0 && view.diagnostics.UnresolvedRows === 0 && view.diagnostics.CorruptDefinitions === 0 && view.diagnostics.UnknownCatalogEntries === 0)

  return (
    <div className="app" onDragOver={(e) => e.preventDefault()} onDrop={onDrop}>
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">☀</span>
          <div>
            <h1>HB SunRise</h1>
            <p>Microsoft Money workstation</p>
          </div>
        </div>

        <nav>
          {(['overview', 'accounts', 'transactions', 'reports', 'validation', 'raw'] as Tab[]).map((key) => (
            <button key={key} className={tab === key ? 'nav active' : 'nav'} disabled={!view && key !== 'overview'} onClick={() => setTab(key)}>
              {key[0].toUpperCase() + key.slice(1)}
              {key === 'validation' && !view && <span className="hint"> </span>}
            </button>
          ))}
        </nav>

        <section className="recent">
          <div className="recent-head">
            <h2>Recent files</h2>
            {recent.length > 0 && (
              <button className="link" onClick={clearRecent} title="Clear the recent files list">
                Clear
              </button>
            )}
          </div>
          {recent.length === 0 && <p className="muted">None yet.</p>}
          {recent.map((path) => (
            <button key={path} className="recent-item" title={path} onClick={() => void open(path)}>
              {path.split(/[\\/]/).pop()}
            </button>
          ))}
        </section>

        <div className="sidebar-foot">
          <button className="link" onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')}>
            {theme === 'light' ? 'Dark theme' : 'Light theme'}
          </button>
        </div>
      </aside>

      <main className="content">
        {!view && (
          <section className="hero" onDragOver={(e) => e.preventDefault()}>
            <div className="hero-card">
              <h1>Open a Microsoft Money file</h1>
              <p className="muted">
                Reads <code>.mny</code> data files and <code>.mbf</code> backups directly. The file is never modified, nothing leaves this
                computer.
              </p>
              <button className="primary" onClick={openDialog} disabled={busy}>
                Choose a file…
              </button>
              <p className="muted small">…or drop a file anywhere on this window.</p>
              {busy && (
                <div className="progress">
                  <div className="progress-track">
                    <div
                      className="progress-fill"
                      style={{ width: progress && progress.Total > 0 ? `${(progress.Done / progress.Total) * 100}%` : '8%' }}
                    />
                  </div>
                  <div className="progress-row">
                    <span className="muted">{progress ? `${progress.Phase}${progress.Total > 0 ? ` · ${progress.Done}/${progress.Total}` : ''}` : 'Working…'}</span>
                    <button className="link" onClick={() => void Cancel()}>
                      Cancel
                    </button>
                  </div>
                </div>
              )}
              {error && <div className="error">{error}</div>}
            </div>
          </section>
        )}

        {view && (
          <>
            <header className="filebar">
              <div>
                <h1>{view.file.name}</h1>
                <p className="muted">
                  {humanSize(view.file.size)} · extracted in {view.elapsed} · {view.accounts.length} accounts · {view.transactions.length}{' '}
                  transactions · {view.skipped} rows left out
                </p>
              </div>
              <div className="filebar-actions">
                <select value={exportKey} onChange={(e) => setExportKey(e.target.value)}>
                  {FORMATS.map((format) => (
                    <option key={format.key} value={format.key}>
                      {format.name}
                    </option>
                  ))}
                </select>
                <button className="primary" onClick={() => void exportTo()}>
                  Export
                </button>
                <button className="secondary" onClick={openDialog} disabled={busy}>
                  Open another…
                </button>
              </div>
            </header>

            {exportNote && <div className={exportNote.startsWith('Export failed') ? 'error' : 'note'}>{exportNote}</div>}
            {!diagnosticsEmpty && (
              <div className="warn">
                Decoding problems: {view.diagnostics.CorruptRows} rows undecodable, {view.diagnostics.UnresolvedRows} rows unresolved,
                {view.diagnostics.CorruptDefinitions > 0 ? ` ${view.diagnostics.CorruptDefinitions} definitions undecodable,` : ''}
                {view.diagnostics.UnknownCatalogEntries > 0 ? ` ${view.diagnostics.UnknownCatalogEntries} catalog entries unreadable,` : ''} see Raw for
                details.
              </div>
            )}

            {tab === 'overview' && (
              <section className="grid">
                <div className="card wide">
                  <h2>File</h2>
                  <dl>
                    <dt>Path</dt>
                    <dd>{view.file.path}</dd>
                    <dt>Size</dt>
                    <dd>{humanSize(view.file.size)}</dd>
                    <dt>SHA-256</dt>
                    <dd className="mono">{view.file.sha256}</dd>
                  </dl>
                </div>
                <div className="card">
                  <h2>Contents</h2>
                  <dl>
                    <dt>Accounts</dt>
                    <dd>{view.accounts.length}</dd>
                    <dt>Transactions</dt>
                    <dd>{view.transactions.length}</dd>
                    <dt>Rows left out</dt>
                    <dd>{view.skipped}</dd>
                  </dl>
                </div>
                <div className="card">
                  <h2>Validation</h2>
                  <dl>
                    <dt>Confirmed</dt>
                    <dd>{confirmed}</dd>
                    <dt>Uncertain</dt>
                    <dd>{uncertain}</dd>
                    <dt>Informational</dt>
                    <dd>{view.validation.length - confirmed - uncertain}</dd>
                  </dl>
                  <button className="link" onClick={() => setTab('validation')}>
                    Open the report →
                  </button>
                </div>
              </section>
            )}

            {tab === 'accounts' && (
              <section className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Account</th>
                      <th>Currency</th>
                    </tr>
                  </thead>
                  <tbody>
                    {view.accounts.map((account) => (
                      <tr key={account.name}>
                        <td>{account.name}</td>
                        <td className="mono">{account.currency}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </section>
            )}

            {tab === 'transactions' && (
              <section className="table-wrap">
                <div className="table-tools">
                  <input placeholder="Search accounts, memos, categories…" value={query} onChange={(e) => setQuery(e.target.value)} />
                  <span className="muted">
                    {transactions.length} of {view.transactions.length} transactions
                  </span>
                </div>
                <div className="virt-head">
                  <span className="sortable" onClick={() => setSort({ field: 'time', dir: sort.field === 'time' ? ((sort.dir * -1) as 1 | -1) : 1 })}>
                    Date {sort.field === 'time' ? (sort.dir === 1 ? '↑' : '↓') : ''}
                  </span>
                  <span>Type</span>
                  <span>Account</span>
                  <span className="amount sortable" onClick={() => setSort({ field: 'amount', dir: sort.field === 'amount' ? ((sort.dir * -1) as 1 | -1) : 1 })}>
                    Amount {sort.field === 'amount' ? (sort.dir === 1 ? '↑' : '↓') : ''}
                  </span>
                  <span>Transfer</span>
                  <span>Category</span>
                  <span>Memo</span>
                </div>
                <div className="virt" ref={virtual.containerRef} onScroll={virtual.onScroll}>
                  <div style={{ height: virtual.totalHeight, position: 'relative' }}>
                    {transactions.slice(virtual.start, virtual.end).map((t, index) => (
                      <div className="virt-row" key={virtual.start + index} style={{ transform: `translateY(${virtual.offsetTop + index * 34}px)` }}>
                        <span className="mono">{t.time}</span>
                        <span>
                          <span className={`chip ${t.type.toLowerCase()}`}>{t.type}</span>
                        </span>
                        <span>{t.account}</span>
                        <span className="mono amount">{amountOf(t.amount)}</span>
                        <span>
                          {t.transferAccount && (
                            <span className="muted">
                              {t.transferAccount}
                              {t.transferAmount > 0 && ` · ${amountOf(t.transferAmount)}`}
                            </span>
                          )}
                        </span>
                        <span>
                          {t.category && <span>{t.category}</span>}
                          {t.category && t.subCategory && <span className="muted"> / {t.subCategory}</span>}
                          {!t.category && t.subCategory && <span>{t.subCategory}</span>}
                        </span>
                        <span className="muted">{t.memo}</span>
                      </div>
                    ))}
                  </div>
                </div>
              </section>
            )}

            {tab === 'reports' && (
              <section className="grid">
                <div className="card wide">
                  <h2>Income versus expenses</h2>
                  {monthly.length > 0 ? <IncomeVersusExpense months={monthly} /> : <p className="muted">No income or expense transactions to chart.</p>}
                </div>
                <div className="card">
                  <h2>Spending by category</h2>
                  {categories.length > 0 ? <CategoryBars slices={categories} /> : <p className="muted">No expenses to chart.</p>}
                </div>
                <div className="card">
                  <h2>Cumulative change from the transactions in the file</h2>
                  <select value={reportAccount} onChange={(e) => setReportAccount(e.target.value)}>
                    <option value="">All accounts</option>
                    {view.accounts.map((account) => (
                      <option key={account.name} value={account.name}>
                        {account.name}
                      </option>
                    ))}
                  </select>
                  <CumulativeLine points={cumulative} />
                  <p className="muted small">
                    Calculated from the transactions this file holds; it is not a Money account balance, because opening balances are not decoded.
                  </p>
                </div>
              </section>
            )}

            {tab === 'validation' && (
              <section className="table-wrap">
                {view.validation.length === 0 && <p className="muted">Validation found nothing to report.</p>}
                {view.validation.map((finding, index) => (
                  <div className="finding" key={index}>
                    <span className={`chip ${finding.severity}`}>{finding.severity}</span>
                    <div>
                      <strong>{finding.kind}</strong>
                      <p>{finding.message}</p>
                      {(finding.account || finding.time) && (
                        <p className="muted">
                          {finding.account} {finding.time} {finding.amount !== 0 && `· ${amountOf(finding.amount)}`}
                        </p>
                      )}
                    </div>
                  </div>
                ))}
              </section>
            )}

            {tab === 'raw' && (
              <section className="grid">
                <div className="card wide">
                  <h2>Decoding diagnostics</h2>
                  {diagnosticsEmpty && <p className="muted">Everything found decoded cleanly.</p>}
                  {!diagnosticsEmpty && (
                    <dl>
                      <dt>Rows undecodable</dt>
                      <dd>{view.diagnostics.CorruptRows}</dd>
                      <dt>Rows unresolved</dt>
                      <dd>{view.diagnostics.UnresolvedRows}</dd>
                      <dt>Definitions undecodable</dt>
                      <dd>{view.diagnostics.CorruptDefinitions}</dd>
                      <dt>Catalog entries unreadable</dt>
                      <dd>{view.diagnostics.UnknownCatalogEntries}</dd>
                    </dl>
                  )}
                  {view.diagnostics.Samples.length > 0 && (
                    <ul className="samples">
                      {view.diagnostics.Samples.map((sample, index) => (
                        <li key={index} className="mono">
                          {sample}
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
                <div className="card">
                  <h2>Source</h2>
                  <dl>
                    <dt>File</dt>
                    <dd>{view.file.name}</dd>
                    <dt>Extracted in</dt>
                    <dd>{view.elapsed}</dd>
                  </dl>
                  <p className="muted small">
                    The original file is only ever read; exports are written where you choose and never overwrite an existing file.
                  </p>
                </div>
              </section>
            )}
          </>
        )}
      </main>
    </div>
  )
}
