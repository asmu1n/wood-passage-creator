import {
  ARTICLE_PROGRESS_EVENT_NAMES,
  type ArticleProgressEvent,
  type ArticleProgressEventName,
  type ImageOutcome,
  type ImageTask,
} from './articleWorkflow'

export const allEventNamesCovered: Exclude<
  ArticleProgressEventName,
  (typeof ARTICLE_PROGRESS_EVENT_NAMES)[number]
> extends never
  ? true
  : never = true

const task: ImageTask = {
  id: 'image-1',
  position: 1,
  slotId: 'cover',
  kind: 'cover',
  brief: 'A woodworking bench',
  preferredMethod: 'PEXELS',
}

const asset = { url: 'https://example.com/bench.jpg', method: 'PEXELS' } as const
const warning = {
  code: 'IMAGE_SKIPPED',
  imageId: 'image-1',
  position: 1,
  message: 'No usable image after bounded attempts',
} as const
const summary = { total: 1, settled: 1, succeeded: 0, skipped: 1 }
const skipped = { task, status: 'skipped', warning } as const satisfies ImageOutcome
const meta = { runId: 'run-1', seq: 1, phase: 'CONTENT_GENERATING' } as const

export const outcomes = [{ task, status: 'succeeded', asset }, skipped] satisfies ImageOutcome[]

export const events = [
  {
    ...meta,
    type: 'task_snapshot',
    data: { status: 'PROCESSING', terminal: false, imageSummary: summary },
  },
  {
    ...meta,
    type: 'titles_done',
    data: { options: [{ mainTitle: 'Title', subTitle: 'Subtitle' }] },
  },
  { ...meta, type: 'outline_snapshot', data: { sections: [{ title: 'Draft' }] } },
  {
    ...meta,
    type: 'outline_done',
    data: { sections: [{ section: 1, title: 'Intro', points: ['Point'] }] },
  },
  { ...meta, type: 'content_delta', data: { delta: 'Text' } },
  { ...meta, type: 'content_generated', data: { content: 'Full body' } },
  { ...meta, type: 'images_planned', data: { tasks: [task] } },
  { ...meta, type: 'image_finished', data: { outcome: skipped, summary } },
  { ...meta, type: 'images_done', data: { outcomes: [skipped], summary } },
  {
    ...meta,
    type: 'content_done',
    data: { content: 'Body', fullContent: 'Body', images: [], warnings: [warning] },
  },
  {
    ...meta,
    type: 'task_error',
    data: { code: 'GENERATION_FAILED', message: 'Unable to generate' },
  },
] satisfies ArticleProgressEvent[]

// @ts-expect-error skipped tasks cannot carry an asset.
export const skippedWithAsset: ImageOutcome = { task, status: 'skipped', warning, asset }

// @ts-expect-error succeeded tasks cannot carry a warning.
export const succeededWithWarning: ImageOutcome = { task, status: 'succeeded', asset, warning }

export const mismatchedEvent: ArticleProgressEvent = {
  ...meta,
  type: 'content_delta',
  data: {
    // @ts-expect-error the event name determines the payload shape.
    sections: [],
  },
}

// @ts-expect-error old raw-JSON outline deltas are not in the new protocol.
export const legacyEvent: ArticleProgressEventName = 'outline_delta'

// @ts-expect-error the internal random-image fallback is not a workflow provider.
export const fallbackTask: ImageTask = { ...task, preferredMethod: 'PICSUM' }
