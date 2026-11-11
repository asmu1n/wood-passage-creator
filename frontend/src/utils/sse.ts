import {
  ARTICLE_PROGRESS_EVENT_NAMES,
  isTerminalArticleProgressEvent,
  type ArticleProgressEvent,
} from '../types/articleWorkflow.ts'
import { parseArticleProgressEvent } from './articleProgress.ts'

export interface SSEOptions {
  onMessage: (message: ArticleProgressEvent) => void
  onError?: (error: Event) => void
  onComplete?: (reason?: string) => void
}

// Stateful only within one subscription; reconnects always begin with a snapshot.
export class ProgressCursor {
  private runId?: string
  private seq = -1
  accept(event: ArticleProgressEvent): 'apply' | 'ignore' | 'resync' {
    if (event.type === 'task_snapshot') {
      if (event.runId === this.runId && event.seq < this.seq) return 'ignore'
      this.runId = event.runId
      this.seq = event.seq
      return 'apply'
    }
    if (event.runId !== this.runId || event.seq > this.seq + 1) return 'resync'
    if (event.seq <= this.seq) return 'ignore'
    this.seq = event.seq
    return 'apply'
  }
}

export class ProgressConnection {
  private source?: EventSource
  private timer?: ReturnType<typeof setTimeout>
  private closed = false
  private retries = 0
  private cursor = new ProgressCursor()
  private taskId: string
  private options: SSEOptions
  constructor(taskId: string, options: SSEOptions) {
    this.taskId = taskId
    this.options = options
    this.open()
  }
  close(): void {
    this.closed = true
    this.source?.close()
    if (this.timer !== undefined) clearTimeout(this.timer)
  }
  private reconnect(): void {
    this.source?.close()
    this.source = undefined
    if (this.closed || this.timer !== undefined) return
    this.retries++
    if (this.retries > 5) {
      this.close()
      this.options.onError?.(new Event('error'))
      return
    }
    this.timer = setTimeout(
      () => {
        this.timer = undefined
        this.open()
      },
      Math.min(500 * 2 ** (this.retries - 1), 5000),
    )
  }
  private open(): void {
    if (this.closed) return
    const source = new EventSource(`/api/article/progress/${encodeURIComponent(this.taskId)}`, {
      withCredentials: true,
    })
    this.source = source
    for (const name of ARTICLE_PROGRESS_EVENT_NAMES) {
      source.addEventListener(name, (raw: MessageEvent) => {
        if (this.closed || source !== this.source) return
        let event: ArticleProgressEvent
        try {
          event = parseArticleProgressEvent(name, raw.data)
        } catch (error) {
          console.error('Invalid article progress event', error)
          this.reconnect()
          return
        }
        const action = this.cursor.accept(event)
        if (action === 'resync') {
          this.reconnect()
          return
        }
        if (action === 'ignore') return
        if (event.type === 'task_snapshot') this.retries = 0
        this.options.onMessage(event)
        if (isTerminalArticleProgressEvent(event)) {
          this.close()
          this.options.onComplete?.(name)
        }
      })
    }
    source.onerror = () => {
      if (!this.closed && source === this.source) this.reconnect()
    }
  }
}

export const connectSSE = (taskId: string, options: SSEOptions): ProgressConnection =>
  new ProgressConnection(taskId, options)
export const closeSSE = (connection: ProgressConnection | null): void => connection?.close()
