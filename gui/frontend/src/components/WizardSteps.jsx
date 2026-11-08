import React from 'react'

// The parts a wizard page (Restore, Undelete, Roll back) is built from: the
// chevron strip of steps, the summary of what has been chosen so far, and
// the Back / Next row. The chevrons look like the Backup Set editor's. A
// step not open yet is greyed, with what it needs first as its tooltip
// (disable, don't hide); an open step, before or after the current one,
// can be clicked. A step is ticked only once it is finished, which is when
// the step after it is open: an open step still to fill in (a destination
// not chosen yet) has no tick. Which steps are open comes from
// stepRules.mjs. (Not named
// wizardSteps.mjs: on Windows that is the same file name as this one, and an
// import without an extension picks .mjs first.)

// steps: [{ label, open, need }] in order; current: 1-based.
export function WizardSteps({ steps, current, onSelect, label }) {
  return (
    <nav className="chev-strip" aria-label={label}>
      {steps.map((s, i) => {
        const n = i + 1
        const finished = s.open && i + 1 < steps.length && steps[i + 1].open
        const state = n === current ? 'active' : finished ? 'done' : s.open ? 'open' : 'todo'
        return (
          <button
            key={n}
            type="button"
            className={`chev wiz ${state}`}
            aria-current={n === current ? 'step' : undefined}
            disabled={!s.open}
            title={s.open ? undefined : s.need}
            onClick={() => { if (s.open && n !== current) onSelect(n) }}
          >
            {state === 'done' && <span aria-hidden="true">✓ </span>}
            {n} {s.label}
          </button>
        )
      })}
    </nav>
  )
}

// items: [[label, value], ...] (a falsy value leaves the item out);
// action: { label, onClick } for the link at the end ("Change").
export function WizardSummary({ items, action }) {
  return (
    <div className="wiz-summary">
      {items.filter(([, v]) => v).map(([k, v]) => (
        <span key={k}><strong>{k}</strong> {v}</span>
      ))}
      {action && (
        <button type="button" className="wiz-link" onClick={action.onClick}>{action.label}</button>
      )}
    </div>
  )
}

// back / next: { label, onClick, disabled, title }; next may instead be any
// element (the Restore button and Stop). extra sits beside Back. className
// is added to the row's own.
export function WizardNav({ back, next, extra, className }) {
  return (
    <div className={className ? `wiz-nav ${className}` : 'wiz-nav'}>
      <div className="wiz-nav-side">
        {back && (
          <button type="button" className="btn btn-secondary" onClick={back.onClick} disabled={back.disabled}>
            ← {back.label}
          </button>
        )}
        {extra}
      </div>
      <div className="wiz-nav-side">
        {next && (React.isValidElement(next) ? next : (
          <button type="button" className="btn" onClick={next.onClick} disabled={next.disabled} title={next.title}>
            {next.label} →
          </button>
        ))}
      </div>
    </div>
  )
}
