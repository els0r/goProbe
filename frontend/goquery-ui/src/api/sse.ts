// Server-Sent Events wire-format parser. Pure, framework-free, owns nothing but
// its own buffer and decoder. Bytes in via push(); complete events out. flush()
// surfaces a trailing, unterminated block when the stream ends without a final
// blank-line delimiter. This is the codebase's test surface for SSE framing.

export interface SSEEvent {
  event?: string
  data?: string
}

// Parse one raw event block (the text between blank-line delimiters) into its
// event/data fields. Comment lines (":") and unrecognized lines are ignored;
// multiple data: lines are joined with newlines, per the SSE spec.
function parseEventBlock(rawEvt: string): SSEEvent {
  const evt: SSEEvent = {}
  const lines = rawEvt.split('\n')
  for (const ln of lines) {
    if (!ln) continue
    if (ln.startsWith(':')) continue // comment
    const m = ln.match(/^(\w+):\s?(.*)$/)
    if (!m) continue
    const k = m[1]
    const v = m[2]
    if (k === 'event') evt.event = v
    else if (k === 'data') evt.data = (evt.data ? evt.data + '\n' : '') + v
  }
  return evt
}

export class SSEParser {
  private decoder = new TextDecoder('utf-8')
  private buf = ''

  // Decode and buffer a chunk of bytes, returning every event now fully framed
  // by a blank line (\n\n). \r\n is normalized to \n first.
  push(chunk: Uint8Array): SSEEvent[] {
    this.buf += this.decoder.decode(chunk, { stream: true })
    this.buf = this.buf.replace(/\r\n/g, '\n')
    const events: SSEEvent[] = []
    let idx
    while ((idx = this.buf.indexOf('\n\n')) >= 0) {
      const rawEvt = this.buf.slice(0, idx)
      this.buf = this.buf.slice(idx + 2)
      events.push(parseEventBlock(rawEvt))
    }
    return events
  }

  // Flush a trailing, unterminated event block at stream end. Returns it only
  // when it carries an event or data field; empty trailing whitespace yields [].
  flush(): SSEEvent[] {
    const rest = this.buf.replace(/\r\n/g, '\n')
    this.buf = ''
    if (!rest || rest.trim().length === 0) return []
    const evt = parseEventBlock(rest)
    if (evt.event || evt.data) return [evt]
    return []
  }
}
