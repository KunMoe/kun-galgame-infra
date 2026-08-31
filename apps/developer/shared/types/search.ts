export interface SearchHeading {
  i: string
  t: string
}

// Single-letter keys because this array is the whole file the palette
// downloads: r=route, t=title, s=section, d=subtitle, b=haystack, h=headings.
export interface SearchEntry {
  r: string
  t: string
  s: string
  d: string
  b: string
  h?: SearchHeading[]
}
