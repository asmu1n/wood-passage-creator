export interface OutlineStreamItem {
  section: number
  title: string
  points: string[]
}

function normalizeSection(item: any, index: number): OutlineStreamItem | null {
  if (!item || typeof item !== 'object') return null
  const title = typeof item.title === 'string' ? item.title : ''
  const points = Array.isArray(item.points)
    ? item.points.filter((p: unknown): p is string => typeof p === 'string')
    : []
  const section =
    typeof item.section === 'number' && Number.isFinite(item.section)
      ? item.section
      : index + 1
  return { section, title, points }
}

function normalizeSections(list: unknown[]): OutlineStreamItem[] {
  const out: OutlineStreamItem[] = []
  list.forEach((item, index) => {
    const section = normalizeSection(item, index)
    if (section) out.push(section)
  })
  return out
}

export function parseOutlineStream(raw: string): OutlineStreamItem[] {
  const str = raw.trim()
  if (!str) return []

  try {
    const parsed = JSON.parse(str)
    if (parsed && Array.isArray(parsed.sections)) {
      return normalizeSections(parsed.sections)
    }
    return []
  } catch {
  }

  try {
    const sectionsMatch = str.match(/"sections"\s*:\s*\[/)
    if (!sectionsMatch || sectionsMatch.index === undefined) return []

    const sectionsStart = str.indexOf('[', sectionsMatch.index)
    if (sectionsStart === -1) return []

    const afterStart = str.substring(sectionsStart)
    const lastBrace = afterStart.lastIndexOf('}')
    if (lastBrace <= 0) return []

    const partialArray = afterStart.substring(0, lastBrace + 1) + ']'
    const parsed = JSON.parse(partialArray)
    if (Array.isArray(parsed)) {
      return normalizeSections(parsed)
    }
    return []
  } catch {
    return []
  }
}
