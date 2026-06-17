// Full reset — lines with display attributes (reverse video, bold, etc.)
// are skipped before highlighting, so \x1b[0m is safe here.
const ANSI_RESET = '\x1b[0m'
// Match ANSI escape sequences: CSI (ESC [ ... letter) and OSC (ESC ] ... BEL/ST)
const ANSI_RE = /(\x1b\[[\x20-\x3F]*[\x40-\x7E]|\x1b[\]PX^_][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[\x20-\x2F][\x30-\x7E]|\x1b[\x30-\x7E])/g

// Split text into segments: alternating [plain, CSI, plain, CSI, ...]
function segmentText(text: string): { text: string; isCSI: boolean }[] {
  const segments: { text: string; isCSI: boolean }[] = []
  let lastEnd = 0
  ANSI_RE.lastIndex = 0
  let m: RegExpExecArray | null
  while ((m = ANSI_RE.exec(text)) !== null) {
    if (m.index > lastEnd) {
      segments.push({ text: text.slice(lastEnd, m.index), isCSI: false })
    }
    segments.push({ text: m[0], isCSI: true })
    lastEnd = m.index + m[0].length
  }
  if (lastEnd < text.length) {
    segments.push({ text: text.slice(lastEnd), isCSI: false })
  }
  if (segments.length === 0) {
    segments.push({ text, isCSI: false })
  }
  return segments
}

// ── Color palette ──
// 256-color SGR codes: \x1b[38;5;{n}m for foreground
// Add '4;' prefix for underline, e.g. \x1b[4;38;5;{n}m
const FG = (n: number) => `\x1b[38;5;${n}m`
const UNDERLINE_FG = (n: number) => `\x1b[4;38;5;${n}m`
const C = {
  url:       UNDERLINE_FG(39),
  ip:        FG(82),
  path:      FG(177),
  datetime:  FG(39),
  string:    FG(215),
  error:     FG(203),
  warning:   FG(221),
  info:      FG(75),
  brace:     FG(223),
  number:    FG(152),
} as const

// Patterns grouped by color type, ordered longest-first
const PATTERNS: { sgr: string; regexes: RegExp[] }[] = [
  { sgr: C.url,     regexes: [/https?:\/\/[^\s\x1b]+/gi] },
  { sgr: C.ip,      regexes: [/\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(:\d+)?\b/g] },
  { sgr: C.path,    regexes: [/(?<=^|\s)(?:\/|~\/)[\w.\/-]+(?=[\s:;"')\]}]|$)/g] },
  { sgr: C.datetime, regexes: [
    /\b\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}(?::\d{2})?(?:[.,]\d+)?Z?\b/g,
    /\b(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun)\s+(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}\s+\d{4}\b/g,
    /\b(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}\b/g,
    /\b\d{2}:\d{2}:\d{2}\b/g,
  ]},
  { sgr: C.string,  regexes: [/"(?:[^"\\]|\\.){2,}"|'(?:[^'\\]|\\.){2,}'/g] },
  { sgr: C.error,   regexes: [/\b(?:ERROR|FAIL(?:ED|URE)?|CRITICAL|FATAL)\b/g] },
  { sgr: C.warning, regexes: [/\bWARN(?:ING)?\b/g] },
  { sgr: C.info,    regexes: [/\b(?:INFO|SUCCESS|OK)\b/g] },
  { sgr: C.brace,   regexes: [/[{}()\[\]|*=<>]/g] },
  { sgr: C.number,  regexes: [/\b\d+\b/g] },
]

function highlightPlainText(text: string): string {
  const segments = segmentText(text)
  let result = ''
  for (const seg of segments) {
    if (seg.isCSI) {
      result += seg.text
    } else {
      type MatchEntry = { start: number; end: number; sgr: string }
      const allMatches: MatchEntry[] = []
      for (const { sgr, regexes } of PATTERNS) {
        for (const regex of regexes) {
          regex.lastIndex = 0
          let m: RegExpExecArray | null
          while ((m = regex.exec(seg.text)) !== null) {
            allMatches.push({ start: m.index, end: m.index + m[0].length, sgr })
            if (allMatches.length > 200) break
          }
          if (allMatches.length > 200) break
        }
        if (allMatches.length > 200) break
      }
      if (allMatches.length > 200) {
        result += seg.text  // pass through unchanged
        continue
      }
      allMatches.sort((a, b) => a.start - b.start || b.end - a.end)
      const filtered: MatchEntry[] = []
      for (const match of allMatches) {
        const last = filtered[filtered.length - 1]
        if (!last || match.start >= last.end) {
          filtered.push(match)
        }
      }
      let highlighted = seg.text
      for (let i = filtered.length - 1; i >= 0; i--) {
        const { start, end, sgr } = filtered[i]
        highlighted = highlighted.slice(0, start) + sgr + highlighted.slice(start, end) + ANSI_RESET + highlighted.slice(end)
      }
      result += highlighted
    }
  }
  return result
}

export function highlight(text: string): string {
  // Process line by line to avoid cross-line regex matches.
  const lines = text.split(/(\r?\n)/)
  let result = ''
  for (const line of lines) {
    if (line === '\r\n' || line === '\n' || line === '\r') {
      result += line
    } else if (line) {
      // Skip lines with display attributes (reverse video, bold,
      // italic, etc. — \x1b[1-8]m). Our injected fg colors would
      // become bg colors under \x1b[7m, washing out the selection.
      if (/\x1b\[[1-8]m/.test(line)) {
        result += line
      } else {
        result += highlightPlainText(line)
      }
    }
  }
  return result
}
