// What the About page and the Preferences welcome box show for the active
// brand (gui/brand.go, via GetBrand). Pure functions, so the rules are tested
// in aboutBrand.test.mjs without rendering anything.

// The project's own repository. Source code, bug reports and the licence always
// point here, whatever the brand: the GPL source offer is the project's.
export const PROJECT_REPO = 'https://github.com/salvanza/salvanza_client'
export const PBS_DOWNLOAD_URL = 'https://www.proxmox.com/en/downloads/proxmox-backup-server'

// A brand with a storage link is selling backup storage: a button with its own
// label, or the standard one when it gives none. Without a link (the default
// build) there is nothing to sell, so the page says how to set up a PBS
// instead and leaves finding a storage provider to the user.
export function storageOffer(brand) {
  const url = (brand && brand.buy_storage_url) || ''
  if (url) return { kind: 'buy', url, text: (brand && brand.buy_storage_text) || '' }
  return { kind: 'selfHost', url: PBS_DOWNLOAD_URL }
}

// The About page links: the brand's own pages, then the project's. The default
// brand's Website is the project's site (salvanza.org), ahead of the
// repository. A URL is listed once, the project's copy winning.
export function aboutLinks(brand) {
  const urls = (brand && brand.urls) || {}
  const project = [
    ['source', PROJECT_REPO],
    ['issues', `${PROJECT_REPO}/issues`],
    ['license', `${PROJECT_REPO}/blob/master/LICENSE`],
  ]
  const seen = new Set(project.map(([, u]) => u))
  const own = [
    ['website', urls.about || (brand && brand.brand_url)],
    ['help', urls.help],
    ['updates', urls.updates || `${PROJECT_REPO}/releases`],
    ['contact', urls.contact],
  ].filter(([, u]) => u && !seen.has(u) && seen.add(u))
  return [...own, ...project]
}
