import type {
  ArticleProgressEvent,
  ArticlePhase,
  ArticleStatus,
  EditorialContext,
  GeneratedImage,
  GenerationWarning,
  GenerationStep,
  ImageSummary,
  OutlinePreviewSection,
  OutlineSection,
  SavedImage,
  TitleOption,
} from '../types/articleWorkflow.ts'

const phases = [
  'PENDING',
  'TITLE_GENERATING',
  'TITLE_SELECTING',
  'OUTLINE_GENERATING',
  'OUTLINE_EDITING',
  'CONTENT_GENERATING',
  'COMPLETED',
]
const methods = ['PEXELS', 'ICONIFY', 'EMOJI_PACK', 'MERMAID', 'SVG_DIAGRAM', 'NANO_BANANA']
const statuses = ['PENDING', 'PROCESSING', 'COMPLETED', 'FAILED']
const steps = ['titles', 'outline', 'content', 'image_plan', 'images', 'merge', 'completed']
const record = (v: unknown): v is Record<string, unknown> =>
  typeof v === 'object' && v !== null && !Array.isArray(v)
const text = (v: unknown): v is string => typeof v === 'string'
const count = (v: unknown): v is number =>
  typeof v === 'number' && Number.isSafeInteger(v) && v >= 0
const list = (v: unknown, valid: (item: unknown) => boolean): boolean =>
  Array.isArray(v) && v.every(valid)
const optional = (
  o: Record<string, unknown>,
  key: string,
  valid: (v: unknown) => boolean,
): boolean => o[key] === undefined || valid(o[key])
const title = (v: unknown): boolean => record(v) && text(v.mainTitle) && text(v.subTitle)
const preview = (v: unknown): boolean =>
  record(v) &&
  optional(v, 'section', count) &&
  optional(v, 'title', text) &&
  optional(v, 'points', (v) => list(v, text))
const section = (v: unknown): boolean =>
  record(v) && count(v.section) && v.section > 0 && text(v.title) && list(v.points, text)
const warning = (v: unknown): boolean =>
  record(v) &&
  v.code === 'IMAGE_SKIPPED' &&
  text(v.imageId) &&
  count(v.position) &&
  v.position > 0 &&
  text(v.message)
const task = (v: unknown): boolean =>
  record(v) &&
  text(v.id) &&
  count(v.position) &&
  v.position > 0 &&
  text(v.slotId) &&
  (v.kind === 'cover' || v.kind === 'illustration') &&
  text(v.brief) &&
  text(v.preferredMethod) &&
  methods.includes(v.preferredMethod)
const asset = (v: unknown): boolean =>
  record(v) && text(v.url) && text(v.method) && methods.includes(v.method)
const image = (v: unknown): boolean => record(v) && task(v.task) && asset(v.asset)
const savedImage = (v: unknown): boolean =>
  record(v) &&
  count(v.position) &&
  text(v.url) &&
  text(v.method) &&
  ['description', 'sectionTitle', 'keywords'].every((key) =>
    optional(v, key, text),
  )
const summary = (v: unknown): boolean =>
  record(v) &&
  count(v.total) &&
  count(v.settled) &&
  count(v.succeeded) &&
  count(v.skipped) &&
  v.settled <= v.total &&
  v.succeeded + v.skipped === v.settled
const outcome = (v: unknown): boolean =>
  record(v) &&
  task(v.task) &&
  ((v.status === 'succeeded' && asset(v.asset) && v.warning === undefined) ||
    (v.status === 'skipped' && warning(v.warning) && v.asset === undefined))
const publicError = (v: unknown): boolean => record(v) && text(v.code) && text(v.message)
const editorial = (v: unknown): boolean =>
  record(v) &&
  text(v.mainTitle) &&
  text(v.subTitle) &&
  text(v.userDescription) &&
  optional(v, 'style', text)

export function isArticleProgressEvent(v: unknown): v is ArticleProgressEvent {
  if (
    !record(v) ||
    !text(v.runId) ||
    !v.runId ||
    !count(v.seq) ||
    !text(v.phase) ||
    !phases.includes(v.phase) ||
    !record(v.data)
  )
    return false
  const d = v.data
  switch (v.type) {
    case 'task_snapshot':
      return (
        text(d.status) &&
        statuses.includes(d.status) &&
        typeof d.terminal === 'boolean' &&
        optional(d, 'topic', text) &&
        summary(d.imageSummary) &&
        optional(d, 'titleOptions', (v) => list(v, title)) &&
        optional(d, 'outline', (v) => list(v, preview)) &&
        optional(d, 'content', text) &&
        optional(d, 'fullContent', text) &&
        optional(d, 'step', (v) => text(v) && steps.includes(v)) &&
        optional(d, 'images', (v) => list(v, image)) &&
        optional(d, 'savedImages', (v) => list(v, savedImage)) &&
        optional(d, 'imageOutcomes', (v) => list(v, outcome)) &&
        optional(d, 'warnings', (v) => list(v, warning)) &&
        optional(d, 'error', publicError) &&
        optional(d, 'editorial', editorial)
      )
    case 'titles_done':
      return list(d.options, title)
    case 'outline_snapshot':
      return list(d.sections, preview)
    case 'outline_done':
      return list(d.sections, section)
    case 'content_delta':
      return text(d.delta)
    case 'content_generated':
      return text(d.content)
    case 'images_planned':
      return list(d.tasks, task)
    case 'image_finished':
      return outcome(d.outcome) && summary(d.summary)
    case 'images_done':
      return list(d.outcomes, outcome) && summary(d.summary)
    case 'content_done':
      return (
        text(d.content) && text(d.fullContent) && list(d.images, image) && list(d.warnings, warning)
      )
    case 'task_error':
      return publicError(d)
    default:
      return false
  }
}

export function parseArticleProgressEvent(name: string, raw: string): ArticleProgressEvent {
  const decoded: unknown = JSON.parse(raw)
  if (!record(decoded)) throw new Error('Invalid progress event envelope')
  const event: unknown = { ...decoded, type: name }
  if (!isArticleProgressEvent(event)) throw new Error(`Invalid progress event: ${name}`)
  return event
}

export interface ArticleProgressState {
  topic: string
  phase: ArticlePhase
  status: ArticleStatus
  terminal: boolean
  step: GenerationStep
  editorial?: EditorialContext
  titleOptions: TitleOption[]
  outline: OutlineSection[]
  outlinePreview: OutlinePreviewSection[]
  content: string
  fullContent: string
  images: SavedImage[]
  warnings: GenerationWarning[]
  summary: ImageSummary
  error?: string
}

export function createArticleProgressState(): ArticleProgressState {
  return {
    topic: '',
    phase: 'PENDING',
    status: 'PENDING',
    terminal: false,
    step: 'titles',
    titleOptions: [],
    outline: [],
    outlinePreview: [],
    content: '',
    fullContent: '',
    images: [],
    warnings: [],
    editorial: undefined,
    error: undefined,
    summary: { total: 0, settled: 0, succeeded: 0, skipped: 0 },
  }
}

export function displayImages(images: GeneratedImage[]): SavedImage[] {
  return images.map(({ task, asset }) => ({
    position: task.position,
    url: asset.url,
    method: asset.method,
    description: task.brief,
  }))
}

export function applyArticleProgressEvent(
  state: ArticleProgressState,
  event: ArticleProgressEvent,
): void {
  state.phase = event.phase
  switch (event.type) {
    case 'task_snapshot': {
      const d = event.data
      Object.assign(state, createArticleProgressState(), {
        phase: event.phase,
        topic: d.topic ?? '',
        status: d.status,
        terminal: d.terminal,
        step: d.step ?? (event.phase === 'COMPLETED' ? 'completed' : 'content'),
        editorial: d.editorial,
        titleOptions: d.titleOptions ?? [],
        outlinePreview: d.outline ?? [],
        content: d.content ?? '',
        fullContent: d.fullContent ?? '',
        images: d.savedImages ?? displayImages(d.images ?? []),
        warnings: d.warnings ?? [],
        summary: d.imageSummary,
        error: d.error?.message,
      })
      if (
        event.phase === 'OUTLINE_EDITING' ||
        event.phase === 'CONTENT_GENERATING' ||
        event.phase === 'COMPLETED'
      ) {
        state.outline = state.outlinePreview.flatMap((s) =>
          s.section !== undefined && s.title !== undefined && s.points !== undefined
            ? [{ section: s.section, title: s.title, points: s.points }]
            : [],
        )
      }
      break
    }
    case 'titles_done':
      state.titleOptions = event.data.options
      state.terminal = true
      break
    case 'outline_snapshot':
      state.outlinePreview = event.data.sections
      state.step = 'outline'
      break
    case 'outline_done':
      state.outline = event.data.sections
      state.outlinePreview = event.data.sections
      state.terminal = true
      break
    case 'content_delta':
      state.content += event.data.delta
      state.step = 'content'
      break
    case 'content_generated':
      state.content = event.data.content
      state.step = 'image_plan'
      break
    case 'images_planned':
      state.summary = { total: event.data.tasks.length, settled: 0, succeeded: 0, skipped: 0 }
      state.images = []
      state.warnings = []
      state.step = 'images'
      break
    case 'image_finished': {
      state.summary = event.data.summary
      const o = event.data.outcome
      if (o.status === 'succeeded')
        state.images.push(...displayImages([{ task: o.task, asset: o.asset }]))
      else state.warnings.push(o.warning)
      break
    }
    case 'images_done':
      state.summary = event.data.summary
      state.step = 'merge'
      state.images = displayImages(
        event.data.outcomes.flatMap((o) =>
          o.status === 'succeeded' ? [{ task: o.task, asset: o.asset }] : [],
        ),
      )
      state.warnings = event.data.outcomes.flatMap((o) =>
        o.status === 'skipped' ? [o.warning] : [],
      )
      break
    case 'content_done':
      state.content = event.data.content
      state.fullContent = event.data.fullContent
      state.images = displayImages(event.data.images)
      state.warnings = event.data.warnings
      state.status = 'COMPLETED'
      state.terminal = true
      state.step = 'completed'
      break
    case 'task_error':
      state.status = 'FAILED'
      state.terminal = true
      state.error = event.data.message
      break
  }
}
