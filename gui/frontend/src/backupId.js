// Suggested Backup IDs for folder Backup Sets: the computer's name, then the
// set's name, so every set of one computer sits together on PBS
// ("deepthought-data", "deepthought-beeby-property") and no two sets share a
// group. PBS allows letters, digits, "_", "-" and "." in a backup ID.

export function slugId(s) {
  return (s || '')
    .normalize('NFKD').replace(/[̀-ͯ]/g, '') // é -> e
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

// suggestBackupId("Deepthought - Data", "DEEPTHOUGHT") -> "deepthought-data"
// suggestBackupId("Beeby Property", "deepthought") -> "deepthought-beeby-property"
// The computer's name is taken out of the set's name wherever it appears
// (start, middle or end) and put in front once. taken: IDs already used by
// other sets; a clash gets "-2", "-3"...
export function suggestBackupId(setName, hostname, taken = []) {
  const host = slugId(hostname) || 'host'
  const hostTokens = host.split('-')
  const tokens = slugId(setName).split('-').filter(Boolean)
  const rest = []
  for (let i = 0; i < tokens.length;) {
    if (hostTokens.every((h, k) => tokens[i + k] === h)) {
      i += hostTokens.length
      continue
    }
    rest.push(tokens[i])
    i++
  }
  let id = rest.length ? `${host}-${rest.join('-')}` : host
  if (id.length > 60) id = id.slice(0, 60).replace(/-+$/, '')
  const used = new Set((taken || []).map(x => (x || '').toLowerCase()))
  let candidate = id
  for (let n = 2; used.has(candidate); n++) candidate = `${id}-${n}`
  return candidate
}
