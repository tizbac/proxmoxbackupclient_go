// Time remaining and Forecast finish are held for 10 s between updates, so
// they do not change with every progress event the way the speed and counts
// do. The forecast is the time the estimate was made plus the time remaining,
// so it stays put while it is held instead of moving with the clock.
export const ETA_HOLD_MS = 10000

// holdEta returns the estimate to show: the new one when there is none held,
// when the held one is 10 s old, when the work is done (0) or when the key
// (a stage) has changed; otherwise the held one. eta is in seconds, at the
// time the estimate was made (ms). null means no estimate yet.
export function holdEta(prev, eta, now, key = '', holdMs = ETA_HOLD_MS) {
  if (eta === null || eta === undefined) return { eta: null, at: 0, key }
  const held = prev && prev.eta !== null && prev.eta !== undefined && prev.at > 0 && prev.key === key
  if (!held || eta === 0 || now - prev.at >= holdMs) return { eta, at: now, key }
  return prev
}

// The forecast finish (ms) of a held estimate.
export const finishAt = h => h.at + h.eta * 1000
