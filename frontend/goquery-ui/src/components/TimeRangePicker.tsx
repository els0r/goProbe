export interface TimeRange {
  start: Date
  end: Date
}

export interface TimeRangePickerProps {
  value: TimeRange
  presets?: { label: string; minutes: number }[]
  onChange: (range: TimeRange) => void
}

const DEFAULT_PRESETS = [5, 15, 60, 360, 1440].map((m) => ({
  label: `Last ${m}m`,
  minutes: m,
}))

export function TimeRangePicker({
  value,
  onChange,
  presets = DEFAULT_PRESETS,
}: TimeRangePickerProps) {
  return (
    <div>
      <select
        onChange={(e) => {
          const p = presets.find((p) => p.label === e.target.value)
          if (p) {
            const end = new Date()
            const start = new Date(Date.now() - p.minutes * 60 * 1000)
            onChange({ start, end })
          }
        }}
        value={''}
      >
        <option value="" disabled>
          Select range
        </option>
        {presets.map((p) => (
          <option key={p.label} value={p.label}>
            {p.label}
          </option>
        ))}
      </select>
      <span>
        {value.start.toISOString()} - {value.end.toISOString()}
      </span>
    </div>
  )
}
