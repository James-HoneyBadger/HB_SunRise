import test from 'node:test'
import assert from 'node:assert/strict'
import { filterLedger, summarizeLedger, emptyLedgerFilter, formatMinor } from '../src/ledger.ts'

const row = (changes = {}) => ({ time:'2026-01-10', type:'Expense', account:'Checking', currency:'USD', amount:125, transferAccount:'', transferAmount:0, category:'Food', subCategory:'', memo:'Morning coffee', payee:'Cafe', split:false, ...changes })
test('account includes incoming transfers once and dates are inclusive', () => {
  const rows = [row(), row({type:'Transfer',account:'Savings',transferAccount:'Checking'}),row({time:'2026-01-11'}),row({account:'Other'})]
  assert.deepEqual(filterLedger(rows,{...emptyLedgerFilter,account:'Checking',from:'2026-01-10',through:'2026-01-10'}),rows.slice(0,2))
})
test('combined text, currency and type filters', () => {
  const rows=[row(),row({currency:'EUR'}),row({type:'Income'}),row({memo:'Tea'})]
  assert.deepEqual(filterLedger(rows,{...emptyLedgerFilter,query:'CAFE coffee',currency:'USD',type:'Expense'}),[rows[0]])
})
test('separate currencies, exclude transfers, exact sums beyond safe integer', () => {
  const rows=[row({type:'Income',amount:Number.MAX_SAFE_INTEGER}),row({type:'Income',amount:2}),row(),row({currency:'EUR'}),row({type:'Transfer',amount:999})]
  const totals=summarizeLedger(rows)
  assert.equal(totals.length,2)
  assert.equal(totals[1].income,9007199254740993n)
  assert.equal(totals[1].expense,125n)
  assert.equal(totals[0].expense,125n)
  assert.equal(formatMinor(-1n),'-0.01')
})
test('unsafe and unknown source values suppress totals', () => {
  assert.equal(summarizeLedger([row({amount:Number.MAX_SAFE_INTEGER+1})])[0].valid,false)
  assert.equal(summarizeLedger([row({currency:''})])[0].valid,false)
})

import { accountHistory } from '../src/history.ts'
test('history refuses unknown incoming transfer rather than inventing a value', () => {
  const transfer = row({type:'Transfer',account:'Savings',transferAccount:'Checking',amount:500,transferAmount:0})
  assert.ok(accountHistory([transfer],'Checking').issue)
  assert.equal(accountHistory([transfer],'Savings').points[0].value,-500)
  assert.ok(accountHistory([transfer],'').issue)
})
test('history uses actual receiving amount and signed source changes', () => {
  const rows=[row({type:'Income',amount:1000}),row({amount:125}),row({type:'Transfer',account:'Savings',transferAccount:'Checking',amount:500,transferAmount:490})]
  assert.deepEqual(accountHistory(rows,'Checking').points.map(p=>p.value),[1000,875,1365])
})
test('history rejects unsafe amounts and cumulative overflow', () => {
  assert.ok(accountHistory([row({amount:Number.MAX_SAFE_INTEGER+1})],'Checking').issue)
  assert.ok(accountHistory([row({type:'Income',amount:Number.MAX_SAFE_INTEGER}),row({type:'Income',amount:1})],'Checking').issue)
})
