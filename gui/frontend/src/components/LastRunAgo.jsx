import { useEffect, useState } from 'react'
import { useTranslation } from '../i18n/i18nContext'
import { formatAgo, msToNextMinute } from '../timeAgo'

// " - 2 days, 20 hours, 45 minutes ago" after a Backup Set's last run time.
// It re-renders only when the whole minutes since the run change, not on
// every app update.
export default function LastRunAgo({ when }) {
  const { t, language } = useTranslation()
  const since = new Date(when).getTime()
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (isNaN(since)) return undefined
    const id = setTimeout(() => setNow(Date.now()), msToNextMinute(since, Date.now()) + 50)
    return () => clearTimeout(id)
  }, [since, now])
  if (isNaN(since)) return null
  return <span>{' - '}{formatAgo(t, language, now - since)}</span>
}
