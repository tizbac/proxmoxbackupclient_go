import { useState, useCallback, useEffect } from 'react'
import { Tree, TreeItem, TreeItemLayout, Spinner } from '@fluentui/react-components'
import { Folder20Regular, Document20Regular } from '@fluentui/react-icons'
import { normPath, isUnder, checkState, computeReincludeExcludes } from './treeSelection'

let ListDirectoryFn = null
if (window.go) {
  ListDirectoryFn = window.go.main.App.ListDirectory
}

// DirectoryTree — an Explorer-style, lazily-loaded folder/file picker
// replacing the old plain-textarea directory list. Two things it deliberately
// does NOT try to do, both backend limits, not oversights:
//   - Select a lone FILE as its own backup root: PBS's PXAR archive format
//     (via this fork's WriteDir) only ever roots an archive at a directory —
//     see pbscommon/pxar.go's own comment "backing up single file is not
//     possible". A file only gets a meaningful checkbox once it's already
//     inside a checked folder, where unchecking it EXCLUDES that one file.
//   - Arbitrary re-inclusion depth: unchecking a folder excludes everything
//     under it; re-checking one specific item back removes JUST as much of
//     that exclusion as needed (the ancestor exclude is replaced by excluding
//     that ancestor's OTHER children instead), which needs every level
//     between the exclude and the item to already be loaded — true by
//     construction, since the tree has to be expanded that far for the
//     checkbox to be clickable at all.
export default function DirectoryTree({ roots, excludes, onChange }) {
  // path -> DirEntry[] | 'loading' | 'error'
  const [childrenByPath, setChildrenByPath] = useState({})
  const [openItems, setOpenItems] = useState(() => new Set())

  const loadChildren = useCallback(async (path) => {
    if (!ListDirectoryFn) return
    const key = normPath(path)
    setChildrenByPath((prev) => ({ ...prev, [key]: 'loading' }))
    try {
      const entries = await ListDirectoryFn(path)
      setChildrenByPath((prev) => ({ ...prev, [key]: entries || [] }))
    } catch (err) {
      console.error('ListDirectory failed for', path, err)
      setChildrenByPath((prev) => ({ ...prev, [key]: 'error' }))
    }
  }, [])

  // Bootstrap the roots (drive letters / "/") on first mount.
  useEffect(() => {
    loadChildren('')
  }, [loadChildren])

  const handleOpenChange = (_e, data) => {
    setOpenItems(data.openItems)
    const path = data.value
    const key = normPath(path)
    if (data.open && childrenByPath[key] === undefined) {
      loadChildren(path)
    }
  }

  // Re-include `path`, which is currently excluded only because an ANCESTOR
  // of it is in `excludes` (a whole-folder exclude covering it). Replace that
  // one ancestor-level exclude with excludes on every SIBLING along the path
  // down to `path`, so only `path` (and its subtree) comes back — everything
  // else the ancestor exclude used to cover stays excluded. Every directory
  // level walked here is guaranteed already loaded: the tree had to be
  // expanded that far for this checkbox to be clickable in the first place.
  const reinclude = (path, ancestorExclude) => {
    onChange({ roots, excludes: computeReincludeExcludes(childrenByPath, excludes, path, ancestorExclude) })
  }

  // Looks up a rendered node's own DirEntry (for its is_dir flag) from
  // whichever parent listing produced it — every node the user can click is
  // necessarily in some loaded listing already.
  const findNode = (path) => {
    for (const kids of Object.values(childrenByPath)) {
      if (!Array.isArray(kids)) continue
      const hit = kids.find((k) => k.path === path)
      if (hit) return hit
    }
    return null
  }

  const handleCheckedChange = (_e, data) => {
    const path = data.value
    const wantChecked = data.checked === true
    const p = normPath(path)
    const coveringRoot = roots.find((r) => isUnder(r, p))

    if (wantChecked) {
      if (!coveringRoot) {
        // Only a directory can become a new backup root (see doc comment) —
        // a bare file click with nothing covering it is a no-op.
        const node = findNode(path)
        if (!node || !node.is_dir) return
        // Fold any already-checked roots living UNDER this one into it —
        // otherwise checking a parent whose child was independently checked
        // (the 'mixed' case) would leave both as separate roots, backing the
        // child up twice (once standalone, once again as part of the parent).
        onChange({
          roots: [...roots.filter((r) => !isUnder(path, r)), path],
          excludes: excludes.filter((e) => !isUnder(path, e)),
        })
        return
      }
      const directAncestorExclude = excludes.find((e) => isUnder(e, p))
      if (directAncestorExclude) {
        reinclude(path, directAncestorExclude)
        return
      }
      // Already covered by a root and not itself excluded. If this node is
      // 'mixed' only because something BELOW it is excluded, clicking its
      // checkbox (Fluent reports mixed->checked as wantChecked=true, same as
      // a fresh check) re-selects everything under it, same convention as
      // BFW's own parent checkbox.
      const nestedExcludes = excludes.filter((e) => isUnder(p, e))
      if (nestedExcludes.length > 0) {
        onChange({ roots, excludes: excludes.filter((e) => !isUnder(p, e)) })
      }
      return
    }

    // Unchecking.
    if (coveringRoot && normPath(coveringRoot) === p) {
      // This IS one of the top-level included roots — drop it entirely.
      onChange({
        roots: roots.filter((r) => normPath(r) !== p),
        excludes: excludes.filter((e) => !isUnder(path, e)),
      })
    } else if (coveringRoot) {
      onChange({ roots, excludes: [...excludes, path] })
    }
  }

  const checkedItems = new Map()
  const seen = new Set()
  const collectChecked = (path) => {
    const key = normPath(path)
    if (seen.has(key)) return
    seen.add(key)
    checkedItems.set(path, checkState(path, roots, excludes))
  }
  // Fluent only needs entries for rendered items; walk everything currently
  // loaded so every visible row gets an explicit (non-default) state.
  Object.entries(childrenByPath).forEach(([, kids]) => {
    if (Array.isArray(kids)) kids.forEach((k) => collectChecked(k.path))
  })

  const renderNode = (node) => {
    const key = normPath(node.path)
    const kids = childrenByPath[key]
    const isBranch = node.is_dir
    const Icon = node.is_dir ? Folder20Regular : Document20Regular

    return (
      <TreeItem key={node.path} value={node.path} itemType={isBranch ? 'branch' : 'leaf'}>
        <TreeItemLayout iconBefore={<Icon />}>{node.name}</TreeItemLayout>
        {isBranch && (
          <Tree>
            {kids === 'loading' && (
              <TreeItem value={node.path + '\u0000loading'} itemType="leaf">
                <TreeItemLayout>
                  <Spinner size="tiny" label="Loading..." />
                </TreeItemLayout>
              </TreeItem>
            )}
            {kids === 'error' && (
              <TreeItem value={node.path + '\u0000error'} itemType="leaf">
                <TreeItemLayout>Unable to read this folder</TreeItemLayout>
              </TreeItem>
            )}
            {Array.isArray(kids) && kids.length === 0 && (
              <TreeItem value={node.path + '\u0000empty'} itemType="leaf">
                <TreeItemLayout>(empty)</TreeItemLayout>
              </TreeItem>
            )}
            {Array.isArray(kids) && kids.map((child) => renderNode(child))}
          </Tree>
        )}
      </TreeItem>
    )
  }

  const rootEntries = childrenByPath[''] // ListDirectory("") result
  const treeRoots = Array.isArray(rootEntries) ? rootEntries : []

  return (
    <div style={{ border: '1px solid #d0d0d0', borderRadius: 4, maxHeight: 320, overflow: 'auto', padding: '4px 0' }}>
      {rootEntries === 'loading' && (
        <div style={{ padding: 12 }}>
          <Spinner size="tiny" label="Loading drives..." />
        </div>
      )}
      {rootEntries === 'error' && <div style={{ padding: 12, color: '#b91c1c' }}>Unable to list drives</div>}
      {treeRoots.length > 0 && (
        <Tree
          aria-label="Backup folder picker"
          selectionMode="multiselect"
          openItems={openItems}
          onOpenChange={handleOpenChange}
          checkedItems={checkedItems}
          onCheckedChange={handleCheckedChange}
        >
          {treeRoots.map((node) => renderNode(node))}
        </Tree>
      )}
    </div>
  )
}
