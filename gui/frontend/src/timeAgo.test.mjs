// node src/timeAgo.test.mjs
import assert from 'node:assert/strict'
import { agoParts, formatAgo, msToNextMinute } from './timeAgo.js'
import translations from './i18n/translations.js'

const MIN = 60 * 1000

assert.deepEqual(agoParts(0), { days: 0, hours: 0, minutes: 0 })
assert.deepEqual(agoParts(59 * 1000), { days: 0, hours: 0, minutes: 0 })
assert.deepEqual(agoParts(((2 * 24 + 20) * 60 + 45) * MIN + 30 * 1000), { days: 2, hours: 20, minutes: 45 })
assert.deepEqual(agoParts(-5 * MIN), { days: 0, hours: 0, minutes: 0 })

// t() as i18nContext has it: the language, then English, then the key.
const tFor = lang => (key, params) => {
  let v = translations[lang]?.[key]
  if (v === undefined) v = translations.en[key]
  if (v === undefined) return key
  for (const [p, x] of Object.entries(params || {})) v = v.split(`{${p}}`).join(String(x))
  return v
}
const span = (d, h, m) => ((d * 24 + h) * 60 + m) * MIN
const en = tFor('en')
assert.equal(formatAgo(en, 'en', span(2, 20, 45)), '2 days, 20 hours, 45 minutes ago')
assert.equal(formatAgo(en, 'en', span(1, 1, 1)), '1 day, 1 hour, 1 minute ago')
assert.equal(formatAgo(en, 'en', span(0, 3, 0)), '3 hours ago')
assert.equal(formatAgo(en, 'en', span(1, 0, 5)), '1 day, 5 minutes ago')
assert.equal(formatAgo(en, 'en', 20 * 1000), 'just now')
assert.equal(formatAgo(tFor('de'), 'de', span(2, 1, 0)), 'vor 2 Tagen, 1 Stunde')
assert.equal(formatAgo(tFor('pl'), 'pl', span(0, 22, 5)), '22 godziny, 5 minut temu')
assert.equal(formatAgo(tFor('cs'), 'cs', span(1, 0, 3)), 'před 1 dnem, 3 minutami')
assert.equal(formatAgo(tFor('ro'), 'ro', span(0, 20, 2)), 'acum 20 de ore, 2 minute')
assert.equal(formatAgo(tFor('lt'), 'lt', span(0, 11, 21)), 'prieš 11 valandų, 21 minutę')

// Every language has every key the phrase uses, with its {n} or {time}.
for (const lang of Object.keys(translations)) {
  const tr = translations[lang]
  for (const k of ['agoPhrase', 'agoJustNow', 'agoJoin', 'agoDays_other', 'agoHours_other', 'agoMinutes_other']) {
    assert.ok(typeof tr[k] === 'string', `${lang} lacks ${k}`)
  }
  assert.ok(tr.agoPhrase.includes('{time}'), `${lang} agoPhrase`)
  for (const k of Object.keys(tr).filter(k => /^ago(Days|Hours|Minutes)_/.test(k))) {
    assert.ok(tr[k].includes('{n}'), `${lang} ${k}`)
  }
  // Each plural category the language uses for 0-200 has a form, or _other.
  const t = tFor(lang)
  for (let n = 1; n <= 200; n++) assert.ok(!/\{|ago[A-Z]/.test(formatAgo(t, lang, span(n, 0, 0))), `${lang} ${n} days`)
}

assert.equal(msToNextMinute(0, 0), MIN)
assert.equal(msToNextMinute(0, 59 * 1000), 1000)
assert.equal(msToNextMinute(10 * 1000, 70 * 1000), MIN)
assert.equal(msToNextMinute(10 * 1000, 0), MIN)
console.log('timeAgo: ok')
