// Backup Set schedules in the editor and on the set card: plain words for a
// schedule, and its next runs for the preview. The next-run maths mirrors
// gui/schedules.go (the backend decides when a set really runs).

import { dateLocale } from '../i18n/i18nContext'

export const WEEK = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']
const WD = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
export const UNITS = ['minutes', 'hours', 'days', 'weeks', 'months', 'years']
export const MIN_REPEAT_MINUTES = 5

const pad = n => String(n).padStart(2, '0')
export const isoDay = d => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`

// newSchedule is a fresh schedule of the given unit with sensible defaults.
export function newSchedule(unit, id) {
  const s = { id, enabled: true, unit, every: 1, start: isoDay(new Date()) }
  switch (unit) {
    case 'minutes': return { ...s, every: 30 }
    case 'hours': return { ...s, every: 2, minute: 0 }
    case 'weeks': return { ...s, at: '01:00', weekdays: ['Sat'] }
    case 'months': return { ...s, at: '03:00', monthMode: 'last', monthDay: 1, nth: 1, nthWeekday: 'Sun' }
    case 'years': return { ...s, at: '23:00', yearMonth: 12, yearDay: 31 }
    default: return { ...s, unit: 'days', at: '02:00' }
  }
}

// jobScheduling returns a set's { mode, schedules }. The backend moves every
// set over when it loads them; a set that still has only the old fields
// (triggerMode and its fields) is read the same way as jobSchedules there.
export function jobScheduling(job) {
  if (job.scheduleMode) return { mode: job.scheduleMode, schedules: job.schedules || [] }
  const days = (job.daysOfWeek || []).length >= 7 ? [] : (job.daysOfWeek || [])
  if (job.triggerMode === 'manual') return { mode: 'manual', schedules: [] }
  if (job.triggerMode === 'interval') {
    if (!(job.intervalMinutes > 0)) return { mode: 'scheduled', schedules: [] }
    const hours = job.intervalMinutes % 60 === 0
    return { mode: 'scheduled', schedules: [{
      id: 's1', enabled: true, unit: hours ? 'hours' : 'minutes', every: hours ? job.intervalMinutes / 60 : job.intervalMinutes,
      windowStart: job.windowAllDay ? '' : job.windowStart, windowEnd: job.windowAllDay ? '' : job.windowEnd, weekdays: days,
    }] }
  }
  if (!job.scheduleTime) return { mode: 'scheduled', schedules: [] }
  return { mode: 'scheduled', schedules: [{ id: 's1', enabled: true, unit: 'days', every: 1, at: job.scheduleTime, weekdays: days }] }
}

// nextScheduleId returns an ID none of the schedules has ("s1", "s2"...).
export function nextScheduleId(list) {
  const used = new Set((list || []).map(s => s.id))
  for (let n = 1; ; n++) if (!used.has(`s${n}`)) return `s${n}`
}

// tp picks the form of key for n in the app's language: key_1 for exactly
// one where there is one ("Every day", no number), else the plural category
// (key_zero, key_one, key_few, key_many, key_other; in some languages "one"
// also covers 21, 31...), falling back to key_other.
export function tp(t, language, key, n, params = {}) {
  const tryKey = k => { const v = t(k, { n, ...params }); return v !== k ? v : null }
  if (n === 1) { const v = tryKey(`${key}_1`); if (v) return v }
  let cat = 'other'
  try { cat = new Intl.PluralRules(language).select(n) } catch { /* unknown language: other */ }
  return tryKey(`${key}_${cat}`) || t(`${key}_other`, { n, ...params })
}

// Weekday and month names in the app's language, from the browser, in the
// computer's regional form of it ("23 October" in en-GB, not "October 23").
const ref = new Date(2024, 0, 1) // a Monday
export const weekdayName = (language, wd, style = 'long') => {
  const d = new Date(ref); d.setDate(1 + WEEK.indexOf(wd))
  try { return d.toLocaleDateString(dateLocale(language), { weekday: style }) } catch { return wd }
}
export const monthName = (language, m) => {
  try { return new Date(2024, m - 1, 1).toLocaleDateString(dateLocale(language), { month: 'long' }) } catch { return String(m) }
}
const dayMonth = (language, m, d) => {
  try { return new Date(2024, m - 1, d).toLocaleDateString(dateLocale(language), { day: 'numeric', month: 'long' }) } catch { return `${d}/${m}` }
}

// nthWord is "2nd", "last"... for a weekday of the month. Where ordinals agree
// with the weekday's gender, a language gives each weekday a gender
// (schedGender_Tue: "f" or "n") and the forms schedNth2_f, schedNthLast_n...;
// the plain schedNth2 is the masculine/default form.
export function nthWord(t, n, weekday) {
  const base = `schedNth${n === -1 ? 'Last' : n}`
  const gk = `schedGender_${weekday}`
  const g = t(gk)
  if (g && g !== gk && g !== 'm') {
    const k = `${base}_${g}`
    const v = t(k)
    if (v !== k) return v
  }
  return t(base)
}

// daysText names a list of weekdays ("Mon to Fri", "weekends", "Mon, Wed").
export function daysText(t, language, days) {
  const o = WEEK.filter(w => (days || []).includes(w))
  if (o.length === 0 || o.length === 7) return ''
  const j = o.join()
  if (j === 'Mon,Tue,Wed,Thu,Fri') return t('schedDaysWeekdays')
  if (j === 'Sat,Sun') return t('schedDaysWeekend')
  return o.map(w => weekdayName(language, w, 'short')).join(', ')
}

// describe puts a schedule in plain words: "Every month on the last day at 03:30".
export function describe(t, language, s) {
  const every = tp(t, language, `schedEvery_${s.unit}`, Math.max(1, s.every | 0))
  const days = daysText(t, language, s.weekdays)
  switch (s.unit) {
    case 'minutes':
    case 'hours': {
      let d = every
      if (s.windowStart && s.windowEnd) d = t('schedDescWindow', { every, start: s.windowStart, end: s.windowEnd })
      else if (s.unit === 'hours' && s.minute) d = t('schedDescAtMinute', { every, mm: pad(s.minute) })
      return days ? t('schedDescOnDays', { desc: d, days }) : d
    }
    case 'days':
      return days ? t('schedDescDaysOnly', { every, time: s.at, days }) : t('schedDescAt', { every, time: s.at })
    case 'weeks':
      return t('schedDescWeeks', { every, days: daysText(t, language, s.weekdays) || t('everyDay'), time: s.at })
    case 'months':
      if (s.monthMode === 'last') return t('schedDescMonthLast', { every, time: s.at })
      if (s.monthMode === 'nth') return t('schedDescMonthNth', { every, nth: nthWord(t, s.nth, s.nthWeekday), weekday: weekdayName(language, s.nthWeekday), time: s.at })
      return t('schedDescMonthDay', { every, d: s.monthDay, time: s.at })
    case 'years':
      return t('schedDescYear', { every, date: dayMonth(language, s.yearMonth, s.yearDay), time: s.at })
  }
  return ''
}

// badgeKey is the translation key of a schedule's badge (DAILY, MONTHLY...).
export const badgeKey = s => ({ minutes: 'triggerModeInterval', hours: 'triggerModeInterval', days: 'triggerModeDaily', weeks: 'schedBadgeWeekly', months: 'schedBadgeMonthly', years: 'schedBadgeYearly' })[s.unit]

// scheduleProblem returns the translation key of what stops a schedule being
// saved, or '' (the backend checks the same in validateSchedules).
export function scheduleProblem(s) {
  if (s.unit === 'minutes' && s.every < MIN_REPEAT_MINUTES) return 'schedMinRepeat'
  if ((s.unit === 'minutes' || s.unit === 'hours') && s.windowStart && s.windowEnd && s.windowEnd <= s.windowStart) return 'schedWindowOrder'
  if (s.unit === 'weeks' && !(s.weekdays || []).length) return 'schedPickDay'
  return ''
}

// ---------- next runs (mirror of schedules.go) ----------
const hm = s => { const [h, m] = String(s || '').split(':').map(Number); return (h || 0) * 60 + (m || 0) }
const dayStart = d => new Date(d.getFullYear(), d.getMonth(), d.getDate())
const civil = d => Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()) / 86400000
const lastDay = (y, m) => new Date(y, m + 1, 0).getDate()
const monday = d => new Date(d.getFullYear(), d.getMonth(), d.getDate() - ((d.getDay() + 6) % 7))
const anchorOf = s => {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s.start || '')
  return m ? new Date(+m[1], +m[2] - 1, +m[3]) : new Date(2000, 0, 3)
}
const listed = (d, days, emptyIsAll) => (!days || days.length === 0) ? emptyIsAll : days.includes(WD[d.getDay()])

function onDay(s, d, a, every) {
  switch (s.unit) {
    case 'minutes': case 'hours': return listed(d, s.weekdays, true)
    case 'days': return (civil(d) - civil(a)) % every === 0 && listed(d, s.weekdays, true)
    case 'weeks': {
      const days = (s.weekdays && s.weekdays.length) ? s.weekdays : [WD[a.getDay()]]
      return listed(d, days, false) && ((civil(monday(d)) - civil(monday(a))) / 7) % every === 0
    }
    case 'months': {
      if (((d.getFullYear() - a.getFullYear()) * 12 + d.getMonth() - a.getMonth()) % every) return false
      const ld = lastDay(d.getFullYear(), d.getMonth()), dd = d.getDate()
      if (s.monthMode === 'day') return dd === Math.min(s.monthDay, ld)
      if (s.monthMode === 'last') return dd === ld
      if (WD[d.getDay()] !== s.nthWeekday) return false
      return s.nth === -1 ? dd + 7 > ld : Math.floor((dd - 1) / 7) + 1 === s.nth
    }
    case 'years':
      if ((d.getFullYear() - a.getFullYear()) % every || d.getMonth() !== s.yearMonth - 1) return false
      return d.getDate() === Math.min(s.yearDay, lastDay(d.getFullYear(), d.getMonth()))
  }
  return false
}

function timesOn(s, d) {
  const at = m => new Date(d.getFullYear(), d.getMonth(), d.getDate(), Math.floor(m / 60), m % 60)
  if (s.unit !== 'minutes' && s.unit !== 'hours') return s.at ? [at(hm(s.at))] : []
  const step = Math.max(1, s.every | 0) * (s.unit === 'hours' ? 60 : 1)
  let from = s.unit === 'hours' ? (s.minute | 0) : 0, to = 24 * 60 - 1
  if (s.windowStart && s.windowEnd) { from = hm(s.windowStart); to = Math.max(from, hm(s.windowEnd)) }
  const out = []
  for (let m = from; m <= to && out.length < 300; m += step) out.push(at(m))
  return out
}

// nextRuns lists the schedule's next n runs after `after`.
export function nextRuns(s, after, n) {
  const out = []
  const a = anchorOf(s)
  const every = Math.max(1, s.every | 0)
  let d0 = dayStart(after)
  if (d0 < a) d0 = a
  const horizon = { days: every + 8, weeks: 7 * every + 8, months: 31 * every + 62, years: 366 * every + 1464 }[s.unit] || 8
  // Enough days for n runs, however far apart they are.
  for (let i = 0; i <= horizon * n && out.length < n; i++) {
    const d = new Date(d0.getFullYear(), d0.getMonth(), d0.getDate() + i)
    if (!onDay(s, d, a, every)) continue
    for (const t of timesOn(s, d)) {
      if (t > after) { out.push(t); if (out.length >= n) break }
    }
  }
  return out
}

// nextSetRun is the set's next run over its enabled schedules: { time, schedule }.
export function nextSetRun(schedules, after = new Date()) {
  let best = null
  for (const s of schedules || []) {
    if (!s.enabled || scheduleProblem(s)) continue
    const [t] = nextRuns(s, after, 1)
    if (t && (!best || t < best.time)) best = { time: t, schedule: s }
  }
  return best
}
