import assert from 'node:assert/strict'
import test from 'node:test'
import {
  parseArticleProgressEvent,
  createArticleProgressState,
  applyArticleProgressEvent,
} from '../../utils/articleProgress.ts'
import { ProgressCursor, connectSSE } from '../../utils/sse.ts'
import { getExecutionStepName } from '../../utils/executionLogs.ts'

const summary = { total: 0, settled: 0, succeeded: 0, skipped: 0 }
const envelope = (type, data, seq = 0, runId = 'run') => ({
  type,
  runId,
  seq,
  phase: 'CONTENT_GENERATING',
  data,
})
const snapshot = (data = {}, seq = 0) =>
  envelope(
    'task_snapshot',
    { status: 'PROCESSING', terminal: false, imageSummary: summary, ...data },
    seq,
  )
const task = {
  id: 'image-1',
  position: 1,
  slotId: 'cover',
  kind: 'cover',
  brief: 'Cover',
  preferredMethod: 'PEXELS',
}
const warning = { code: 'IMAGE_SKIPPED', imageId: 'image-1', position: 1, message: 'Cover skipped' }

test('parses only complete typed envelopes and rejects old protocol', () => {
  const parsed = parseArticleProgressEvent(
    'content_delta',
    JSON.stringify(envelope('task_error', { delta: 'Body' }, 1)),
  )
  assert.equal(parsed.type, 'content_delta')
  for (const raw of [
    { delta: 'legacy' },
    envelope('content_delta', { delta: 1 }, 1),
    { ...snapshot(), seq: Number.MAX_SAFE_INTEGER + 1 },
    envelope('images_done', { outcomes: [], summary: { ...summary, settled: 1 } }, 1),
  ])
    assert.throws(() => parseArticleProgressEvent(raw.type ?? 'content_delta', JSON.stringify(raw)))
  assert.throws(() =>
    parseArticleProgressEvent(
      'outline_delta',
      JSON.stringify(envelope('outline_delta', { delta: '{}' }, 1)),
    ),
  )
})

test('outline snapshots replace previews instead of appending or guessing JSON', () => {
  const state = createArticleProgressState()
  applyArticleProgressEvent(
    state,
    envelope('outline_snapshot', { sections: [{ title: 'Half' }] }, 1),
  )
  applyArticleProgressEvent(
    state,
    envelope(
      'outline_snapshot',
      { sections: [{ section: 1, title: 'Complete', points: ['Point'] }] },
      2,
    ),
  )
  assert.deepEqual(state.outlinePreview, [{ section: 1, title: 'Complete', points: ['Point'] }])
})

test('snapshots repair missing deltas, clear omitted fields and preserve saved images', () => {
  const state = createArticleProgressState()
  applyArticleProgressEvent(state, envelope('content_delta', { delta: 'Lost prefix' }, 1))
  const saved = [{ position: 1, method: 'PICSUM', url: 'https://picsum.photos/1' }]
  const incoming = snapshot({ content: 'Authoritative body', savedImages: saved }, 8)
  assert.equal(
    parseArticleProgressEvent(incoming.type, JSON.stringify(incoming)).data.content,
    'Authoritative body',
  )
  applyArticleProgressEvent(state, incoming)
  assert.equal(state.content, 'Authoritative body')
  assert.deepEqual(state.images, saved)
  applyArticleProgressEvent(state, snapshot({}, 9))
  assert.equal(state.content, '')
  assert.deepEqual(state.images, [])
})

test('skipped images count as settled and zero-image batches are well-defined', () => {
  const state = createArticleProgressState()
  applyArticleProgressEvent(state, envelope('images_planned', { tasks: [task] }, 1))
  const next = { total: 1, settled: 1, succeeded: 0, skipped: 1 }
  applyArticleProgressEvent(
    state,
    envelope('image_finished', { outcome: { task, status: 'skipped', warning }, summary: next }, 2),
  )
  assert.equal(state.summary.settled, 1)
  assert.equal(state.status, 'PENDING')
  assert.deepEqual(state.warnings, [warning])
  applyArticleProgressEvent(
    state,
    envelope('images_done', { outcomes: [{ task, status: 'skipped', warning }], summary: next }, 3),
  )
  assert.equal(state.step, 'merge')
  applyArticleProgressEvent(state, envelope('images_planned', { tasks: [] }, 4))
  assert.deepEqual(state.summary, summary)
})

test('cursor rejects duplicate, out-of-order, gapped and cross-run deltas', () => {
  const cursor = new ProgressCursor()
  assert.equal(cursor.accept(envelope('content_delta', { delta: 'x' }, 1)), 'resync')
  assert.equal(cursor.accept(snapshot({}, 5)), 'apply')
  assert.equal(cursor.accept(envelope('content_delta', { delta: 'x' }, 6)), 'apply')
  assert.equal(cursor.accept(envelope('content_delta', { delta: 'x' }, 6)), 'ignore')
  assert.equal(cursor.accept(envelope('content_delta', { delta: 'x' }, 9)), 'resync')
  assert.equal(cursor.accept(envelope('content_delta', { delta: 'x' }, 7, 'another')), 'resync')
  assert.equal(cursor.accept(snapshot({}, 9)), 'apply')
})

class FakeEventSource extends EventTarget {
  static instances = []
  closed = false
  constructor(url) {
    super()
    this.url = url
    FakeEventSource.instances.push(this)
  }
  close() {
    this.closed = true
  }
  send(event) {
    this.dispatchEvent(new MessageEvent(event.type, { data: JSON.stringify(event) }))
  }
}

test('connection reconnects after loss and terminal snapshots close it immediately', (t) => {
  const original = globalThis.EventSource
  globalThis.EventSource = FakeEventSource
  FakeEventSource.instances = []
  t.mock.timers.enable({ apis: ['setTimeout'] })
  const received = []
  let completed = 0
  const connection = connectSSE('task/with space', {
    onMessage: (event) => received.push(event),
    onComplete: () => completed++,
  })
  t.after(() => {
    connection.close()
    globalThis.EventSource = original
  })
  const first = FakeEventSource.instances[0]
  assert.match(first.url, /task%2Fwith%20space/)
  first.send(snapshot({ content: 'First' }))
  first.send(envelope('content_delta', { delta: 'gap' }, 3))
  assert.equal(first.closed, true)
  t.mock.timers.tick(500)
  const second = FakeEventSource.instances[1]
  second.send(
    snapshot({ content: 'Repaired', status: 'COMPLETED', terminal: true, fullContent: 'Final' }, 3),
  )
  assert.equal(second.closed, true)
  assert.equal(completed, 1)
  assert.equal(received.length, 2)
})

test('execution log names preserve both Genkit and historical records', () => {
  assert.equal(
    getExecutionStepName('title_generator'),
    getExecutionStepName('agent1_generate_titles'),
  )
  assert.equal(getExecutionStepName('outline_modifier'), getExecutionStepName('ai_modify_outline'))
  assert.equal(getExecutionStepName('content_merger'), '图文合成')
  assert.equal(getExecutionStepName('future_step'), 'future_step')
})
