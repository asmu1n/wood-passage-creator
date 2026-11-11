import assert from 'node:assert/strict'
import test from 'node:test'

import { ARTICLE_PROGRESS_EVENT_NAMES, isTerminalArticleProgressEvent } from '../articleWorkflow.ts'

test('new protocol has exactly the named business events', () => {
  assert.deepEqual(ARTICLE_PROGRESS_EVENT_NAMES, [
    'task_snapshot',
    'titles_done',
    'outline_snapshot',
    'outline_done',
    'content_delta',
    'content_generated',
    'images_planned',
    'image_finished',
    'images_done',
    'content_done',
    'task_error',
  ])
})

test('a skipped image finishes one item, not the article stage', () => {
  for (const type of ARTICLE_PROGRESS_EVENT_NAMES) {
    const event = { type, data: { terminal: false } }
    const terminal = ['titles_done', 'outline_done', 'content_done', 'task_error'].includes(type)
    assert.equal(isTerminalArticleProgressEvent(event), terminal, type)
  }
})

test('a fresh terminal snapshot closes a late or reconnected subscription', () => {
  assert.equal(
    isTerminalArticleProgressEvent({ type: 'task_snapshot', data: { terminal: true } }),
    true,
  )
})
