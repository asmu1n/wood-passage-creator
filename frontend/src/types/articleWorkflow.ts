// Named SSE business events shared with internal/app/article/workflow/events.go.
export type ArticlePhase =
  | 'PENDING'
  | 'TITLE_GENERATING'
  | 'TITLE_SELECTING'
  | 'OUTLINE_GENERATING'
  | 'OUTLINE_EDITING'
  | 'CONTENT_GENERATING'
  | 'COMPLETED'

export type ArticleStatus = 'PENDING' | 'PROCESSING' | 'COMPLETED' | 'FAILED'
export type GenerationStep =
  | 'titles'
  | 'outline'
  | 'content'
  | 'image_plan'
  | 'images'
  | 'merge'
  | 'completed'

export type ImageMethod =
  | 'PEXELS'
  | 'ICONIFY'
  | 'EMOJI_PACK'
  | 'MERMAID'
  | 'SVG_DIAGRAM'
  | 'NANO_BANANA'

export interface TitleOption {
  mainTitle: string
  subTitle: string
}

export interface OutlineSection {
  section: number
  title: string
  points: string[]
}

export interface OutlinePreviewSection {
  section?: number
  title?: string
  points?: string[]
}

export interface PlannedImage {
  slotId: string
  kind: 'cover' | 'illustration'
  brief: string
  preferredMethod: ImageMethod
}

export interface ImageTask extends PlannedImage {
  id: string
  position: number
}

export interface ImageAsset {
  /** Image/data URL or the existing mermaid: source representation. */
  url: string
  method: ImageMethod
}

export interface GeneratedImage {
  task: ImageTask
  asset: ImageAsset
}

export interface GenerationWarning {
  code: 'IMAGE_SKIPPED'
  imageId: string
  position: number
  message: string
}

export type ImageOutcome =
  | { task: ImageTask; status: 'succeeded'; asset: ImageAsset; warning?: never }
  | { task: ImageTask; status: 'skipped'; warning: GenerationWarning; asset?: never }

export interface ImageSummary {
  total: number
  settled: number
  succeeded: number
  skipped: number
}

export interface ArticleResult {
  content: string
  fullContent: string
  images: GeneratedImage[]
  warnings: GenerationWarning[]
}

export interface TaskError {
  code: string
  message: string
}

export interface TaskSnapshot {
  topic?: string
  status: ArticleStatus
  terminal: boolean
  titleOptions?: TitleOption[]
  outline?: OutlinePreviewSection[]
  content?: string
  fullContent?: string
  images?: GeneratedImage[]
  imageOutcomes?: ImageOutcome[]
  imageSummary: ImageSummary
  warnings?: GenerationWarning[]
  error?: TaskError
  editorial?: EditorialContext
  step?: GenerationStep
  savedImages?: SavedImage[]
}

export interface EditorialContext {
  mainTitle: string
  subTitle: string
  style?: string
  userDescription: string
}

export interface SavedImage {
  position: number
  url: string
  method: string
  keywords?: string
  sectionTitle?: string
  description?: string
}

export interface ArticleProgressPayloads {
  task_snapshot: TaskSnapshot
  titles_done: { options: TitleOption[] }
  outline_snapshot: { sections: OutlinePreviewSection[] }
  outline_done: { sections: OutlineSection[] }
  content_delta: { delta: string }
  content_generated: { content: string }
  images_planned: { tasks: ImageTask[] }
  image_finished: { outcome: ImageOutcome; summary: ImageSummary }
  images_done: { outcomes: ImageOutcome[]; summary: ImageSummary }
  content_done: ArticleResult
  task_error: TaskError
}

export type ArticleProgressEventName = keyof ArticleProgressPayloads

export const ARTICLE_PROGRESS_EVENT_NAMES = [
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
] as const satisfies readonly ArticleProgressEventName[]

// The frame's event: name is added as type by the client, not inside wire JSON.
export interface ArticleProgressEnvelope<Name extends ArticleProgressEventName> {
  runId: string
  seq: number
  phase: ArticlePhase
  data: ArticleProgressPayloads[Name]
}

export type ArticleProgressEvent = {
  [Name in ArticleProgressEventName]: ArticleProgressEnvelope<Name> & { type: Name }
}[ArticleProgressEventName]

export function isTerminalArticleProgressEvent(event: ArticleProgressEvent): boolean {
  switch (event.type) {
    case 'task_snapshot':
      return event.data.terminal
    case 'titles_done':
    case 'outline_done':
    case 'content_done':
    case 'task_error':
      return true
    default:
      return false
  }
}
