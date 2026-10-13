import { useState } from 'react'
import { useTranslation } from '../i18n/i18nContext'
import {
  WEEK, UNITS, MIN_REPEAT_MINUTES, newSchedule, nextScheduleId, describe, nextRuns,
  nextSetRun, scheduleProblem, weekdayName, monthName, tp,
} from './schedules'

// The Schedule tab of the Backup Set editor: "On a schedule" with a list of
// schedules (each in plain words, with Edit, Delete and an on/off tick), or
// "Only when I click Run Now". Editing a schedule shows only the fields its
// unit needs and its next 3 runs.
export default function ScheduleEditor({ scheduleMode, setScheduleMode, schedules, setSchedules, runAtStartup, setRunAtStartup }) {
  const { t, language } = useTranslation()
  const [editingId, setEditingId] = useState(null)
  const editing = schedules.find(s => s.id === editingId)

  const update = (id, patch) => setSchedules(schedules.map(s => s.id === id ? { ...s, ...patch } : s))
  const add = (unit) => {
    const s = newSchedule(unit, nextScheduleId(schedules))
    setSchedules([...schedules, s])
    setEditingId(s.id)
  }
  const remove = (id) => {
    setSchedules(schedules.filter(s => s.id !== id))
    if (editingId === id) setEditingId(null)
  }
  const fmt = d => d.toLocaleString(undefined, { weekday: 'short', day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })
  const setNext = scheduleMode === 'manual' ? null : nextSetRun(schedules)
  const enabledCount = schedules.filter(s => s.enabled).length

  const chip = (selected) => ({
    padding: '6px 10px', border: 'none', borderRadius: '6px', cursor: 'pointer', fontSize: '13px',
    backgroundColor: selected ? 'var(--accent)' : '#e2e8f0', color: selected ? 'white' : '#4a5568',
    fontWeight: selected ? 'bold' : 'normal',
  })
  const row = { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '8px', margin: '10px 0' }
  const key = { minWidth: '110px', color: '#5a6478' }
  const input = { padding: '6px 8px', fontSize: '14px', width: 'auto', margin: 0 }
  const radioLbl = { display: 'inline-flex', gap: '6px', alignItems: 'center', fontWeight: 'normal', margin: 0, cursor: 'pointer' }
  const hint = { fontSize: '12px', color: '#6c757d' }
  const warn = { fontSize: '12px', color: '#9a5b00', background: '#fff4e0', padding: '6px 8px', borderRadius: '5px' }

  const dayChips = (s) => (
    <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }} role="group" aria-label={t('daysOfWeekLabel')}>
      {WEEK.map(day => {
        const selected = (s.weekdays || []).includes(day)
        return (
          <button key={day} type="button" aria-pressed={selected} style={chip(selected)}
            onClick={() => update(s.id, { weekdays: selected ? s.weekdays.filter(d => d !== day) : [...(s.weekdays || []), day] })}>
            {t(`day${day}`)}
          </button>
        )
      })}
    </div>
  )

  const editor = (s) => {
    const problem = scheduleProblem(s)
    const runs = problem ? [] : nextRuns(s, new Date(), 3)
    const windowed = !!(s.windowStart && s.windowEnd)
    return (
      <div style={{ marginTop: '14px', border: '1px solid var(--accent)', borderRadius: '8px', padding: '14px' }} id="schedule-editor">
        <strong>{t('schedEditTitle')}</strong>
        <div style={row}>
          <span style={key}>{t('schedRepeatEvery')}</span>
          <input id="sched-every" type="number" min={s.unit === 'minutes' ? MIN_REPEAT_MINUTES : 1} value={s.every} style={{ ...input, width: '80px' }}
            onChange={e => update(s.id, { every: Math.max(1, parseInt(e.target.value, 10) || 1) })} />
          <select id="sched-unit" value={s.unit} style={input} aria-label={t('schedUnitLabel')}
            onChange={e => {
              const unit = e.target.value
              // A fresh schedule of the new unit, keeping what carries over.
              const fresh = newSchedule(unit, s.id)
              const keepDays = unit === 'weeks' ? ((s.weekdays || []).length ? s.weekdays : fresh.weekdays) : (s.weekdays || [])
              setSchedules(schedules.map(x => x.id === s.id
                ? { ...fresh, enabled: s.enabled, start: s.start, at: fresh.at && s.at ? s.at : fresh.at, weekdays: keepDays }
                : x))
            }}>
            {UNITS.map(u => <option key={u} value={u}>{t(`schedUnit_${u}`)}</option>)}
          </select>
        </div>

        {(s.unit === 'minutes' || s.unit === 'hours') && (
          <>
            <div style={row} role="radiogroup" aria-label={t('schedWhen')}>
              <span style={key}>{t('schedWhen')}</span>
              <label style={radioLbl}>
                <input type="radio" name="sched-window" id="sched-allday" checked={!windowed}
                  onChange={() => update(s.id, { windowStart: '', windowEnd: '' })} /> {t('windowAllDay')}
              </label>
              <label style={radioLbl}>
                <input type="radio" name="sched-window" id="sched-between" checked={windowed}
                  onChange={() => update(s.id, { windowStart: '08:00', windowEnd: '18:00' })} /> {t('windowBetween')}
              </label>
              <input id="sched-win-start" type="time" value={s.windowStart || '08:00'} disabled={!windowed} style={input} aria-label={t('schedFrom')}
                onChange={e => e.target.value && update(s.id, { windowStart: e.target.value })} />
              <span>{t('windowAnd')}</span>
              <input id="sched-win-end" type="time" value={s.windowEnd || '18:00'} disabled={!windowed} style={input} aria-label={t('schedTo')}
                onChange={e => e.target.value && update(s.id, { windowEnd: e.target.value })} />
            </div>
            {s.unit === 'hours' && !windowed && (
              <div style={row}>
                <span style={key}>{t('schedAtMinute')}</span>
                <input id="sched-minute" type="number" min="0" max="59" value={s.minute || 0} style={{ ...input, width: '70px' }}
                  onChange={e => update(s.id, { minute: Math.min(59, Math.max(0, parseInt(e.target.value, 10) || 0)) })} />
                <span>{t('schedPastHour')}</span>
              </div>
            )}
            <div style={row}><span style={key}>{t('schedOnlyOn')}</span>{dayChips(s)}</div>
            <div style={hint}>{t('schedNoDayEveryDay')}</div>
          </>
        )}

        {s.unit === 'days' && (
          <>
            <div style={row}><span style={key}>{t('schedAt')}</span>
              <input id="sched-at" type="time" value={s.at} style={input} onChange={e => e.target.value && update(s.id, { at: e.target.value })} /></div>
            <div style={row}><span style={key}>{t('schedOnlyOn')}</span>{dayChips(s)}</div>
            <div style={hint}>{t('schedNoDayEveryDay')}</div>
          </>
        )}

        {s.unit === 'weeks' && (
          <>
            <div style={row}><span style={key}>{t('schedOn')}</span>{dayChips(s)}</div>
            <div style={row}><span style={key}>{t('schedAt')}</span>
              <input id="sched-at" type="time" value={s.at} style={input} onChange={e => e.target.value && update(s.id, { at: e.target.value })} /></div>
          </>
        )}

        {s.unit === 'months' && (
          <>
            <div role="radiogroup" aria-label={t('schedOn')}>
              <div style={row}>
                <span style={key}>{t('schedOn')}</span>
                <label style={radioLbl}>
                  <input type="radio" name="sched-mm" id="sched-mm-day" checked={s.monthMode === 'day'} onChange={() => update(s.id, { monthMode: 'day' })} /> {t('schedMonthDay')}
                </label>
                <input id="sched-monthday" type="number" min="1" max="31" value={s.monthDay || 1} disabled={s.monthMode !== 'day'} style={{ ...input, width: '70px' }}
                  aria-label={t('schedMonthDay')} onChange={e => update(s.id, { monthDay: Math.min(31, Math.max(1, parseInt(e.target.value, 10) || 1)) })} />
              </div>
              <div style={row}><span style={key}></span>
                <label style={radioLbl}>
                  <input type="radio" name="sched-mm" id="sched-mm-last" checked={s.monthMode === 'last'} onChange={() => update(s.id, { monthMode: 'last' })} /> {t('schedMonthLast')}
                </label>
              </div>
              <div style={row}><span style={key}></span>
                <label style={radioLbl}>
                  <input type="radio" name="sched-mm" id="sched-mm-nth" checked={s.monthMode === 'nth'} onChange={() => update(s.id, { monthMode: 'nth' })} /> {t('schedMonthThe')}
                </label>
                <select id="sched-nth" value={s.nth || 1} disabled={s.monthMode !== 'nth'} style={input} aria-label={t('schedWhich')}
                  onChange={e => update(s.id, { nth: Number(e.target.value) })}>
                  {[1, 2, 3, 4, -1].map(n => <option key={n} value={n}>{t(`schedNth${n === -1 ? 'Last' : n}`)}</option>)}
                </select>
                <select id="sched-nthwd" value={s.nthWeekday || 'Sun'} disabled={s.monthMode !== 'nth'} style={input} aria-label={t('daysOfWeekLabel')}
                  onChange={e => update(s.id, { nthWeekday: e.target.value })}>
                  {WEEK.map(w => <option key={w} value={w}>{weekdayName(language, w)}</option>)}
                </select>
              </div>
            </div>
            <div style={row}><span style={key}>{t('schedAt')}</span>
              <input id="sched-at" type="time" value={s.at} style={input} onChange={e => e.target.value && update(s.id, { at: e.target.value })} /></div>
            {s.monthMode === 'day' && s.monthDay > 28 && <div style={hint}>{t('schedShortMonthHint', { d: s.monthDay })}</div>}
          </>
        )}

        {s.unit === 'years' && (
          <>
            <div style={row}><span style={key}>{t('schedOn')}</span>
              <input id="sched-yearday" type="number" min="1" max="31" value={s.yearDay} style={{ ...input, width: '70px' }} aria-label={t('schedMonthDay')}
                onChange={e => update(s.id, { yearDay: Math.min(31, Math.max(1, parseInt(e.target.value, 10) || 1)) })} />
              <select id="sched-yearmonth" value={s.yearMonth} style={input} aria-label={t('schedMonth')}
                onChange={e => update(s.id, { yearMonth: Number(e.target.value) })}>
                {Array.from({ length: 12 }, (_, i) => <option key={i + 1} value={i + 1}>{monthName(language, i + 1)}</option>)}
              </select>
            </div>
            <div style={row}><span style={key}>{t('schedAt')}</span>
              <input id="sched-at" type="time" value={s.at} style={input} onChange={e => e.target.value && update(s.id, { at: e.target.value })} /></div>
            {s.yearMonth === 2 && s.yearDay === 29 && <div style={hint}>{t('schedFeb29Hint')}</div>}
          </>
        )}

        {s.every > 1 && ['days', 'weeks', 'months', 'years'].includes(s.unit) && (
          <div style={row}><span style={key}>{t('schedStarting')}</span>
            <input id="sched-start" type="date" value={s.start || ''} style={input} onChange={e => e.target.value && update(s.id, { start: e.target.value })} />
            <span style={hint}>{tp(t, language, `schedCountsFrom_${s.unit}`, s.every)}</span>
          </div>
        )}

        {problem && <div style={warn}>{t(problem, { n: MIN_REPEAT_MINUTES })}</div>}

        <div style={{ marginTop: '12px', paddingTop: '10px', borderTop: '1px dashed #d8dee9', fontSize: '13px' }}>
          <strong>{t('schedNext3')}</strong>
          {runs.length > 0
            ? <ol style={{ margin: '6px 0 0', paddingLeft: '20px', fontVariantNumeric: 'tabular-nums' }}>{runs.map(d => <li key={d.getTime()}>{fmt(d)}</li>)}</ol>
            : !problem && <div style={hint}>{t('schedNoneSoon')}</div>}
        </div>
        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '8px' }}>
          <button type="button" className="btn btn-secondary" id="sched-done" onClick={() => setEditingId(null)}>{t('schedDone')}</button>
        </div>
      </div>
    )
  }

  return (
    <div>
      <div className="form-group">
        <label id="sched-mode-label">{t('schedWhenLabel')}</label>
        <div role="radiogroup" aria-labelledby="sched-mode-label" style={{ display: 'flex', flexWrap: 'wrap', gap: '6px 20px', marginTop: '8px' }}>
          <label style={{ ...radioLbl, gap: '8px' }}>
            <input type="radio" name="sched-mode" id="sched-mode-scheduled" checked={scheduleMode !== 'manual'} onChange={() => setScheduleMode('scheduled')} />
            {t('schedModeScheduled')}
          </label>
          <label style={{ ...radioLbl, gap: '8px' }}>
            <input type="radio" name="sched-mode" id="sched-mode-manual" checked={scheduleMode === 'manual'} onChange={() => setScheduleMode('manual')} />
            {t('schedModeManual')}
          </label>
        </div>
      </div>

      {scheduleMode !== 'manual' && (
        <>
          <div className="form-group">
            <label>{t('schedListLabel')}</label>
            <ul style={{ listStyle: 'none', margin: '8px 0 0', padding: 0, border: '1px solid #d8dee9', borderRadius: '8px', overflow: 'hidden' }} id="schedule-list">
              {schedules.length === 0 && <li style={{ padding: '10px 12px', color: '#6c757d', fontSize: '14px' }}>{t('schedNone')}</li>}
              {schedules.map((s, i) => (
                <li key={s.id} className="schedule-row" style={{
                  display: 'grid', gridTemplateColumns: 'auto minmax(0, 1fr) auto', alignItems: 'center', gap: '10px', padding: '9px 12px',
                  borderTop: i === 0 ? 'none' : '1px solid #e6e6e6', background: s.id === editingId ? 'rgba(61, 90, 168, 0.08)' : 'transparent',
                }}>
                  <input type="checkbox" id={`sched-on-${s.id}`} checked={s.enabled} aria-label={t('schedOnLabel')}
                    onChange={e => update(s.id, { enabled: e.target.checked })} style={{ width: '18px', height: '18px' }} />
                  <span style={{ minWidth: 0, fontSize: '14px', color: s.enabled ? 'inherit' : '#8a93a5' }}>
                    {describe(t, language, s)}
                    {!s.enabled && <span style={{ fontSize: '12px' }}> ({t('schedOff')})</span>}
                    {scheduleProblem(s) && <span style={{ fontSize: '12px', color: '#9a5b00' }}> ({t(scheduleProblem(s), { n: MIN_REPEAT_MINUTES })})</span>}
                  </span>
                  <span style={{ display: 'flex', gap: '4px' }}>
                    <button type="button" className="btn btn-secondary" style={{ padding: '4px 10px', fontSize: '13px' }} onClick={() => setEditingId(s.id === editingId ? null : s.id)}>{t('edit')}</button>
                    <button type="button" className="btn btn-secondary" style={{ padding: '4px 10px', fontSize: '13px' }} onClick={() => remove(s.id)}>{t('delete')}</button>
                  </span>
                </li>
              ))}
            </ul>
            <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '6px', marginTop: '10px', fontSize: '13px', color: '#6c757d' }}>
              <button type="button" className="btn btn-secondary" id="sched-add" onClick={() => add('days')}>{t('schedAdd')}</button>
              <span>{t('schedAddFrom')}</span>
              <button type="button" className="btn btn-secondary" id="sched-add-weekly" onClick={() => add('weeks')}>{t('schedBadgeWeekly')}</button>
              <button type="button" className="btn btn-secondary" id="sched-add-monthly" onClick={() => add('months')}>{t('schedBadgeMonthly')}</button>
              <button type="button" className="btn btn-secondary" id="sched-add-yearly" onClick={() => add('years')}>{t('schedBadgeYearly')}</button>
            </div>
            {editing && editor(editing)}
          </div>

          <div className="form-group">
            <label style={{ display: 'flex', alignItems: 'center', gap: '10px', cursor: 'pointer', fontWeight: 'normal' }}>
              <input type="checkbox" id="sched-startup" checked={runAtStartup} onChange={e => setRunAtStartup(e.target.checked)} style={{ width: '20px', height: '20px', cursor: 'pointer' }} />
              <span>{t('runAtStartup')}</span>
            </label>
          </div>
        </>
      )}

      <div className="info-box" style={{ backgroundColor: '#eef2ff' }} id="schedule-summary">
        {scheduleMode === 'manual'
          ? <>{t('schedulingInfoManual')}{schedules.length > 0 && <><br />{t('schedManualKept')}</>}</>
          : setNext
            ? <>{t('schedSetNext', { time: fmt(setNext.time) })} ({describe(t, language, setNext.schedule)})
                {enabledCount > 1 && <><br />{tp(t, language, 'schedCountOn', enabledCount)}</>}
                {runAtStartup && <><br />{t('andAtStartup')}</>}
                <br />{t('schedCatchUp')}</>
            : <>{t('schedNoneOn')}{runAtStartup && <><br />{t('andAtStartup')}</>}</>}
      </div>
    </div>
  )
}
