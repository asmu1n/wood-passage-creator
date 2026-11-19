import assert from 'node:assert/strict'
import test from 'node:test'
import { connectSSE } from '../../utils/sse.ts'

test('connection stops after five consecutive reconnect attempts', (t) => {
  const original = globalThis.EventSource
  const sources = []
  class UnavailableEventSource extends EventTarget {
    constructor() {
      super()
      sources.push(this)
    }
    close() {}
  }
  globalThis.EventSource = UnavailableEventSource
  t.mock.timers.enable({ apis: ['setTimeout'] })
  let errors = 0
  const connection = connectSSE('task', {
    onMessage: () => assert.fail('unexpected message'),
    onError: () => errors++,
  })
  t.after(() => {
    connection.close()
    globalThis.EventSource = original
  })
  for (let attempt = 0; attempt < 6; attempt++) {
    sources.at(-1).onerror(new Event('error'))
    t.mock.timers.tick(5000)
  }
  assert.equal(sources.length, 6)
  assert.equal(errors, 1)
  connection.close()
  t.mock.timers.tick(20000)
  assert.equal(sources.length, 6)
})
