interface ChevronProps {
  /** Rotate 180° to indicate an open/expanded state */
  open?: boolean
  className?: string
}

// Canonical down-chevron used across all dropdown-style controls.
export function Chevron({ open, className }: ChevronProps) {
  return (
    <svg
      className={
        `h-3.5 w-3.5 shrink-0 text-gray-400 transition-transform ${open ? 'rotate-180' : ''} ` +
        (className ?? '')
      }
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden="true"
    >
      <path
        fillRule="evenodd"
        d="M5.23 7.21a.75.75 0 0 1 1.06.02L10 11.06l3.71-3.83a.75.75 0 1 1 1.08 1.04l-4.25 4.39a.75.75 0 0 1-1.08 0L5.23 8.27a.75.75 0 0 1 .02-1.06Z"
        clipRule="evenodd"
      />
    </svg>
  )
}
