import React from 'react'

// A small fixed set of choices (up to three) as radio buttons: every option
// stays visible and is one click away, where a drop-down takes two.
// options: [{ value, label }]. vertical stacks long labels one per line.
export default function RadioGroup({ name, value, onChange, options, disabled = false, labelId, vertical = false }) {
  return (
    <div
      role="radiogroup"
      aria-labelledby={labelId}
      aria-disabled={disabled || undefined}
      style={{
        display: 'flex',
        flexDirection: vertical ? 'column' : 'row',
        gap: vertical ? '6px' : '24px',
        flexWrap: 'wrap',
        opacity: disabled ? 0.55 : 1,
      }}
    >
      {options.map((o) => (
        <label
          key={o.value}
          style={{display: 'flex', alignItems: 'center', gap: '6px', margin: 0, fontWeight: 'normal', cursor: disabled ? 'not-allowed' : 'pointer'}}
        >
          <input
            type="radio"
            name={name}
            value={o.value}
            checked={value === o.value}
            disabled={disabled}
            onChange={() => onChange(o.value)}
            style={{width: 'auto', margin: 0}}
          />
          {o.label}
        </label>
      ))}
    </div>
  )
}
