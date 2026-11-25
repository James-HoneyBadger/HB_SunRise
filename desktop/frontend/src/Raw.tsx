// The raw database inspection workspace: header, page map, table catalog,
// schemas and raw record pages, straight from the MSISAM engine.

import { useCallback, useEffect, useState } from 'react'
import { Inspect, InspectRows } from '../wailsjs/go/main/App'
import type { main, msmoney } from '../wailsjs/go/models'
import type { FileInfo } from './bindings'

type Inspection = main.inspectionView
type Rows = main.rowsView

const PAGE_SIZE = 100

interface RawProps {
  file: FileInfo
}

export default function RawInspector({ file }: RawProps) {
  const [inspection, setInspection] = useState<Inspection | null>(null)
  const [error, setError] = useState('')
  const [selected, setSelected] = useState('')
  const [rows, setRows] = useState<Rows | null>(null)
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    setBusy(true)
    setError('')
    try {
      const result = (await Inspect(file.path)) as unknown as Inspection
      setInspection(result)
      setSelected(result.tables[0]?.name ?? '')
    } catch (err) {
      setError(String(err))
    } finally {
      setBusy(false)
    }
  }, [file.path])

  useEffect(() => {
    setInspection(null)
    setRows(null)
    setSelected('')
    void load()
  }, [load])

  const loadRows = useCallback(
    async (table: string, offset: number) => {
      if (!table) return
      setBusy(true)
      try {
        const page = (await InspectRows(table, offset, PAGE_SIZE)) as unknown as Rows
        setRows(page)
      } catch (err) {
        setError(String(err))
      } finally {
        setBusy(false)
      }
    },
    [],
  )

  useEffect(() => {
    if (selected) void loadRows(selected, 0)
  }, [selected, loadRows])

  const schema = inspection?.tables.find((table) => table.name === selected)

  return (
    <section className="grid">
      {error && <div className="card wide error">{error}</div>}

      <div className="card">
        <h2>Database</h2>
        {!inspection && <p className="muted">{busy ? 'Reading…' : 'Nothing inspected yet.'}</p>}
        {inspection && (
          <dl>
            <dt>Engine version</dt>
            <dd>{inspection.version}</dd>
            <dt>Code page</dt>
            <dd>{inspection.codePage}</dd>
            <dt>Language ID</dt>
            <dd>{inspection.languageID}</dd>
            <dt>Pages</dt>
            <dd>{inspection.pageCount}</dd>
          </dl>
        )}
      </div>

      <div className="card">
        <h2>Page map</h2>
        {!inspection && <p className="muted">—</p>}
        {inspection && (
          <dl>
            {inspection.pageTypeCounts.map((entry) => (
              <span className="page-count" key={entry.type}>
                <dt>{entry.name}</dt>
                <dd>{entry.count}</dd>
              </span>
            ))}
          </dl>
        )}
      </div>

      <div className="card">
        <h2>Tables</h2>
        {!inspection && <p className="muted">—</p>}
        {inspection && (
          <ul className="table-list">
            {inspection.tables.map((table) => (
              <li key={table.name}>
                <button className={selected === table.name ? 'table-name active' : 'table-name'} onClick={() => setSelected(table.name)}>
                  <strong>{table.name}</strong>
                  <span className="muted">
                    {table.rowCount} rows · {table.columns.length} columns · page {table.definitionPage}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="card wide">
        <h2>Schema — {schema?.name ?? 'select a table'}</h2>
        {schema && (
          <table>
            <thead>
              <tr>
                <th>Column</th>
                <th>Type</th>
                <th>Number</th>
                <th>Offset</th>
                <th>Size</th>
              </tr>
            </thead>
            <tbody>
              {schema.columns.map((column) => (
                <tr key={column.name}>
                  <td>{column.name}</td>
                  <td className="mono">{column.type}</td>
                  <td className="mono">{column.number}</td>
                  <td className="mono">{column.fixed ? column.offset : 'variable'}</td>
                  <td className="mono">{column.size}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="card wide">
        <div className="records-head">
          <h2>Raw records — {rows?.table ?? schema?.name ?? ''}</h2>
          <span className="muted">
            declared rows: {rows?.declaredRows ?? schema?.rowCount ?? 0} · showing {rows ? `${rows.offset}–${rows.offset + rows.rows.length}` : '—'}
          </span>
          <span className="spacer" />
          {rows && rows.offset > 0 && (
            <button className="secondary" onClick={() => void loadRows(selected, Math.max(0, rows.offset - PAGE_SIZE))}>
              Previous
            </button>
          )}
          {rows && rows.rows.length === PAGE_SIZE && (
            <button className="secondary" onClick={() => void loadRows(selected, rows.offset + PAGE_SIZE)}>
              Next
            </button>
          )}
        </div>

        {rows && rows.diagnostics && !diagnosticsEmpty(rows.diagnostics) && (
          <div className="warn">
            Decoding problems on this page walk: {rows.diagnostics.UnresolvedRows} rows unresolved, {rows.diagnostics.CorruptRows} rows undecodable.
          </div>
        )}

        {!rows && <p className="muted">{busy ? 'Reading records…' : 'No records loaded.'}</p>}

        {rows && rows.rows.length === 0 && <p className="muted">No rows in this range.</p>}

        {rows &&
          rows.rows.map((record, index) => (
            <div className="record" key={index}>
              <span className="record-index mono">{rows.offset + index}</span>
              <div className="record-fields">
                {record.map((field, fieldIndex) => (
                  <span className="field" key={fieldIndex} title={`${field.column} (${field.type})`}>
                    <strong className="mono">{field.column}</strong>
                    <span className={field.null ? 'field-null muted' : 'field-value mono'}>
                      {field.null ? '∅' : field.value.length > 200 ? field.value.slice(0, 197) + '…' : field.value}
                    </span>
                  </span>
                ))}
              </div>
            </div>
          ))}
      </div>
    </section>
  )
}

function diagnosticsEmpty(d: msmoney.ExtractedDiagnostics): boolean {
  return d.CorruptRows === 0 && d.UnresolvedRows === 0 && d.CorruptDefinitions === 0 && d.UnknownCatalogEntries === 0
}
