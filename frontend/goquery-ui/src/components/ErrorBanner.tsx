import React from 'react'

export interface ErrorBannerProps {
  message: string
  onDismiss?: () => void
}

export function ErrorBanner({ message, onDismiss }: ErrorBannerProps) {
  return (
    <div role="alert" style={{ background: '#fee', border: '1px solid #f99', padding: 8 }}>
      <span>{message}</span>
      {onDismiss && (
        <button onClick={onDismiss} style={{ marginLeft: 8 }}>
          ×
        </button>
      )}
    </div>
  )
}
