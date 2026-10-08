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

// editDistance counts the single-letter changes (insert, delete, replace,
// swap two neighbours) between a and b, stopping early once over max.
function editDistance(a, b, max) {
  if (Math.abs(a.length - b.length) > max) return max + 1
  let prev2 = null
  let prev = Array.from({ length: b.length + 1 }, (_, j) => j)
  for (let i = 1; i <= a.length; i++) {
    const cur = [i]
    let rowMin = i
    for (let j = 1; j <= b.length; j++) {
      const cost = a[i - 1] === b[j - 1] ? 0 : 1
      let v = Math.min(prev[j] + 1, cur[j - 1] + 1, prev[j - 1] + cost)
      if (prev2 && i > 1 && j > 1 && a[i - 1] === b[j - 2] && a[i - 2] === b[j - 1]) v = Math.min(v, prev2[j - 2] + 1)
      cur.push(v)
      rowMin = Math.min(rowMin, v)
    }
    if (rowMin > max) return max + 1
    prev2 = prev
    prev = cur
  }
  return prev[b.length]
}

// A word of the set's name stands for a word of the computer's name when it
// is the same, or, for longer names, a typing slip away from it:
// "Deepthough" or "Deepthgouht" for deepthought.
function sameWord(word, hostWord) {
  if (word === hostWord) return true
  if (!word || hostWord.length < 5) return false
  return editDistance(word, hostWord, hostWord.length >= 10 ? 2 : 1) <= (hostWord.length >= 10 ? 2 : 1)
}

// suggestBackupId("Deepthought - Data", "DEEPTHOUGHT") -> "deepthought-data"
// suggestBackupId("Beeby Property", "deepthought") -> "deepthought-beeby-property"
// The computer's name is taken out of the set's name wherever it appears
// (start, middle or end, spelt right or nearly so) and put in front once.
// taken: IDs already used by other sets; a clash gets "-2", "-3"...
export function suggestBackupId(setName, hostname, taken = []) {
  const host = slugId(hostname) || 'host'
  const hostTokens = host.split('-')
  const tokens = slugId(setName).split('-').filter(Boolean)
  const rest = []
  for (let i = 0; i < tokens.length;) {
    if (hostTokens.every((h, k) => sameWord(tokens[i + k], h))) {
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
