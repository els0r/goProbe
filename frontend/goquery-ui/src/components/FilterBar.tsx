import { useState } from 'react'

export interface FilterBarProps {
  value: string
  onChange: (v: string) => void
  onSubmit?: () => void
}

export function FilterBar({ value, onChange, onSubmit }: FilterBarProps) {
  const [local, setLocal] = useState(value)
  return (
    <form onSubmit={e => { e.preventDefault(); onChange(local); onSubmit?.() }}>
      <input
        type="text"
        placeholder="filter: sip=10.0.0.1 proto=6"
        value={local}
        onChange={e => setLocal(e.target.value)}
        style={{ width: '32rem' }}
      />
      <button type="submit">Apply</button>
    </form>
  )
}
