import React from 'react'

export interface DetailListProps<T> {
  items: T[]
  loading: boolean
  emptyText: string
  renderItem: (item: T, index: number) => React.ReactNode
}

export function DetailList<T>({ items, loading, emptyText, renderItem }: DetailListProps<T>) {
  return (
    <div className="scroll-thin max-h-[45vh] overflow-auto pr-1">
      {loading && <div className="py-8 text-center text-data text-gray-400">Loading…</div>}
      {!loading && items.length === 0 && (
        <div className="py-8 text-center text-data text-gray-400">{emptyText}</div>
      )}
      <div className="space-y-2">
        {items.map((item, i) => (
          <React.Fragment key={i}>{renderItem(item, i)}</React.Fragment>
        ))}
      </div>
    </div>
  )
}
