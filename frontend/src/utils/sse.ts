/**
 * SSE 工具：对接 Go 后端 named events（event: + data JSON）
 */

export interface SSEMessage {
  /** 事件名，如 titles_done / outline_delta / task_error */
  type: string
  [key: string]: any
}

export interface SSEOptions {
  onMessage: (message: SSEMessage) => void
  onError?: (error: Event) => void
  onComplete?: (reason?: string) => void
}

/** 后端会推送的命名事件 */
export const SSE_EVENT_NAMES = [
  'connected',
  'titles_done',
  'outline_delta',
  'outline_done',
  'content_delta',
  'content_generated',
  'images_planned',
  'image_complete',
  'images_done',
  'merge_done',
  'content_done',
  'task_error',
] as const

/** 本段进度流结束（与后端 IsTerminalSSEEvent 对齐） */
export function isTerminalSSEEvent(name: string): boolean {
  return (
    name === 'titles_done' ||
    name === 'outline_done' ||
    name === 'content_done' ||
    name === 'task_error'
  )
}

/**
 * 建立 SSE 连接（需登录 cookie；开发态走同源 /api 代理）
 */
export const connectSSE = (taskId: string, options: SSEOptions): EventSource => {
  const { onMessage, onError, onComplete } = options
  let intentionalClose = false

  const url = `/api/article/progress/${encodeURIComponent(taskId)}`
  const eventSource = new EventSource(url, { withCredentials: true } as EventSourceInit)

  const handleNamed = (name: string) => (event: Event) => {
    const me = event as MessageEvent
    let payload: Record<string, any> = {}
    try {
      payload = me.data ? JSON.parse(me.data) : {}
    } catch (err) {
      console.error('SSE 消息解析失败:', name, me.data, err)
      return
    }
    const message: SSEMessage = { type: name, ...payload }
    onMessage(message)

    if (isTerminalSSEEvent(name)) {
      intentionalClose = true
      eventSource.close()
      onComplete?.(name)
    }
  }

  for (const name of SSE_EVENT_NAMES) {
    eventSource.addEventListener(name, handleNamed(name))
  }

  // 兜底：无 event 名的 message（一般不会走到）
  eventSource.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      const type = data.type || data.event || 'message'
      onMessage({ type, ...data })
    } catch (error) {
      console.error('SSE default message 解析失败:', error)
    }
  }

  eventSource.onerror = (error) => {
    if (intentionalClose || eventSource.readyState === EventSource.CLOSED) {
      return
    }
    console.error('SSE 连接错误:', error)
    onError?.(error)
    intentionalClose = true
    eventSource.close()
  }

  return eventSource
}

export const closeSSE = (eventSource: EventSource | null) => {
  if (eventSource) {
    eventSource.close()
  }
}
