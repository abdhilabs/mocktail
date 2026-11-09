import { useState } from 'react'
import type { RequestRule, RulesConfig } from '../../lib/mocks'

const blank = (): RequestRule => ({ statusCode: 200, response: {} })

export function RulesTab({ rules, setRules }: { rules: RulesConfig; setRules: (rules: RulesConfig) => void }) {
  const [error, setError] = useState<string | null>(null)
  function update(index: number, patch: Partial<RequestRule>) {
    setRules(rules.map((rule, i) => (i === index ? { ...rule, ...patch } : rule)))
  }
  function editResponse(index: number, text: string) {
    try { update(index, { response: JSON.parse(text) }); setError(null) }
    catch { setError(`Rule ${index + 1}: response must be valid JSON`) }
  }
  return <div className="space-y-3 text-[12px]">
    <div className="text-muted">Rules run top to bottom. The first match wins. Leave the list empty for the static response.</div>
    {rules.map((rule, index) => <div key={index} className="space-y-2 rounded border border-border p-3">
      <div className="flex items-center gap-2"><strong>Rule {index + 1}</strong><button className="ml-auto text-error" onClick={() => setRules(rules.filter((_, i) => i !== index))}>Remove</button></div>
      <div className="grid grid-cols-2 gap-2">
        <input className="rounded border border-border bg-surface p-2" placeholder="Method (optional)" value={rule.method ?? ''} onChange={e => update(index, { method: (e.target.value || undefined) as RequestRule['method'] })} />
        <input className="rounded border border-border bg-surface p-2" placeholder="Path (optional)" value={rule.path ?? ''} onChange={e => update(index, { path: e.target.value || undefined })} />
      </div>
      <textarea className="h-16 w-full rounded border border-border bg-surface p-2 font-mono" value={JSON.stringify({ query: rule.query ?? {}, headers: rule.headers ?? {} }, null, 2)} onChange={e => { try { const v = JSON.parse(e.target.value); update(index, { query: v.query, headers: v.headers }); setError(null) } catch { setError(`Rule ${index + 1}: matchers must be valid JSON`) } }} />
      <div className="flex items-center gap-2"><label>Status</label><input type="number" min="100" max="599" className="w-20 rounded border border-border bg-surface p-2" value={rule.statusCode} onChange={e => update(index, { statusCode: Number(e.target.value) })} /></div>
      <textarea className="h-24 w-full rounded border border-border bg-surface p-2 font-mono" value={JSON.stringify(rule.response, null, 2)} onChange={e => editResponse(index, e.target.value)} />
      {error && <div className="text-error">{error}</div>}
      {index > 0 && <button className="text-muted" onClick={() => { const next = [...rules]; [next[index - 1], next[index]] = [next[index], next[index - 1]]; setRules(next) }}>Move up</button>}
      {index < rules.length - 1 && <button className="ml-3 text-muted" onClick={() => { const next = [...rules]; [next[index], next[index + 1]] = [next[index + 1], next[index]]; setRules(next) }}>Move down</button>}
    </div>)}
    <button className="rounded bg-accent px-3 py-2 text-accent-on" onClick={() => setRules([...rules, blank()])}>Add rule</button>
  </div>
}
