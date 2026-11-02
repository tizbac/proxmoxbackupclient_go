// How long ago a Backup Set last ran, for its card: "2 days, 20 hours, 45
// minutes ago". Whole minutes only (the card updates once a minute), units
// that are zero left out, under a minute is "just now".
//
// Each unit's words come from translations.js in the case the language's
// "ago" phrase needs (German "vor 2 Tagen", Czech "před 2 dny"), with one key
// per plural category: agoDays_one, agoDays_few, agoDays_many, agoDays_other.
// A category a language does not give falls back to _other.

const MINUTE = 60 * 1000

// agoParts splits a span in milliseconds into whole days, hours and minutes.
export function agoParts(ms) {
  const mins = Math.max(0, Math.floor(ms / MINUTE))
  return { days: Math.floor(mins / 1440), hours: Math.floor((mins % 1440) / 60), minutes: mins % 60 }
}

function unit(t, language, key, n) {
  let cat = 'other'
  try { cat = new Intl.PluralRules(language).select(n) } catch { /* unknown language: other */ }
  const v = t(`${key}_${cat}`, { n })
  return v !== `${key}_${cat}` ? v : t(`${key}_other`, { n })
}

// formatAgo is the whole phrase for a span in milliseconds, in the language
// t() speaks. A span below zero (a clock that moved back) counts as just now.
export function formatAgo(t, language, ms) {
  const { days, hours, minutes } = agoParts(ms)
  const parts = []
  if (days) parts.push(unit(t, language, 'agoDays', days))
  if (hours) parts.push(unit(t, language, 'agoHours', hours))
  if (minutes) parts.push(unit(t, language, 'agoMinutes', minutes))
  if (!parts.length) return t('agoJustNow')
  return t('agoPhrase', { time: parts.join(t('agoJoin')) })
}

// msToNextMinute is how long until the shown phrase next changes: the next
// whole minute since `since`, so the card ticks over on time rather than up to
// a minute late.
export function msToNextMinute(since, now) {
  const elapsed = now - since
  if (elapsed < 0) return MINUTE
  return MINUTE - (elapsed % MINUTE)
}
