// node src/aboutBrand.test.mjs
import assert from 'node:assert/strict'
import { storageOffer, aboutLinks, PROJECT_REPO, PBS_DOWNLOAD_URL } from './aboutBrand.js'
import translations from './i18n/translations.js'

// The default brand as gui/brand.go has it (GetBrand JSON).
const def = {
  name: 'proxmoxbackupclient', title: 'Proxmox Backup Client', is_default: true,
  brand_url: 'https://www.proxmox.com/', buy_storage_url: '', buy_storage_text: '',
  urls: {
    about: 'https://www.proxmox.com/', help: 'https://forum.proxmox.com/',
    updates: `${PROJECT_REPO}/releases`, contact: PROJECT_REPO,
  },
}
const nimbus = {
  name: 'nimbusbackup', title: 'Nimbus Backup', is_default: false,
  brand_url: 'https://nimbus.example/', buy_storage_url: 'https://store.nimbus.example/backup',
  buy_storage_text: 'Get Nimbus storage',
  urls: {
    about: 'https://nimbus.example/', help: 'https://nimbus.example/help',
    updates: 'https://nimbus.example/releases', contact: 'https://nimbus.example/contact',
  },
}

// Storage: a brand with a shop sells storage, under its own label or the
// standard one; the default build has nothing to sell and points at PBS.
assert.deepEqual(storageOffer(nimbus), { kind: 'buy', url: 'https://store.nimbus.example/backup', text: 'Get Nimbus storage' })
assert.deepEqual(storageOffer({ ...nimbus, buy_storage_text: '' }).text, '')
assert.deepEqual(storageOffer(def), { kind: 'selfHost', url: PBS_DOWNLOAD_URL })
assert.deepEqual(storageOffer({ ...nimbus, buy_storage_url: '' }).kind, 'selfHost')
assert.deepEqual(storageOffer(null).kind, 'selfHost')

// Links: the default build gets no Website (its "about" is Proxmox's site) and
// its contact, being the repo, shows once as Source code.
const keys = b => aboutLinks(b).map(([k]) => k)
assert.deepEqual(keys(def), ['help', 'updates', 'source', 'issues', 'license'])
assert.equal(aboutLinks(def).find(([k]) => k === 'updates')[1], `${PROJECT_REPO}/releases`)
// A brand's own pages come first; source, issues and licence stay the project's.
assert.deepEqual(keys(nimbus), ['website', 'help', 'updates', 'contact', 'source', 'issues', 'license'])
assert.equal(aboutLinks(nimbus).find(([k]) => k === 'source')[1], PROJECT_REPO)
// A brand with only a home page: Website from brand_url, Releases from the project.
assert.deepEqual(aboutLinks({ is_default: false, brand_url: 'https://v.example/', urls: {} }).slice(0, 2),
  [['website', 'https://v.example/'], ['updates', `${PROJECT_REPO}/releases`]])

// Every language has the new strings, the sentence keeps exactly one {pbs},
// and the old fork-only keys are gone.
const added = ['storageSelfHost', 'buyBackupStorage', 'legalWarranty', 'aboutLinkWebsite', 'aboutLinkHelp', 'aboutLinkContact']
const removed = ['orderStorage', 'downloadPBS', 'legalAlpha', 'legalCredit']
assert.equal(Object.keys(translations).length, 18)
for (const [lang, tr] of Object.entries(translations)) {
  for (const k of added) assert.ok(tr[k], `${lang}: ${k} missing`)
  for (const k of removed) assert.equal(tr[k], undefined, `${lang}: ${k} still there`)
  assert.equal(tr.storageSelfHost.split('{pbs}').length, 2, `${lang}: storageSelfHost needs one {pbs}`)
  assert.ok(!/mjb-is|fork|homelab|Workgroups|BFW/i.test(tr.copyright + tr.legalWarranty), `${lang}: fork wording left`)
}
console.log('aboutBrand: all passed')
