/**
 * The hairline that separates one stacked segment from the next is a stroke on
 * the rect, so it has to survive both extremes of this data. At 2px, and drawn
 * on the bottom segment only, a 3px 5xx sliver became stroke with no fill left
 * and detached from the bar as a floating tick — and the bottom segment came
 * out visibly narrower than the ones above it. At 1px on every segment both go
 * away: nothing under ~3px is distorted and every segment is inset equally.
 */
export const CHART_STACK_GAP = 1

/** Breathing room between adjacent bars, as a share of the category width. */
export const CHART_BAR_CATEGORY_GAP = '28%'
