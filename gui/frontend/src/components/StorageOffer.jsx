import { useTranslation } from '../i18n/i18nContext'
import { storageOffer } from '../aboutBrand'

// The storage offer for the active brand (see storageOffer in aboutBrand.js):
// a branded build's "Buy Backup Storage" button, or for the default build one
// sentence with Proxmox Backup Server as the only link.
export default function StorageOffer({ brand, style }) {
  const { t } = useTranslation()
  const offer = storageOffer(brand)
  if (offer.kind === 'buy') {
    return (
      <a className="cta-btn" href={offer.url} target="_blank" data-external="true" rel="noopener noreferrer" style={style}>
        {offer.text || t('buyBackupStorage')}
      </a>
    )
  }
  const [before, after = ''] = t('storageSelfHost').split('{pbs}')
  return (
    <span style={style}>
      {before}
      <a href={offer.url} target="_blank" data-external="true" rel="noopener noreferrer"
        style={{color: 'var(--accent)', fontWeight: 600, textDecoration: 'underline'}}>Proxmox Backup Server</a>
      {after}
    </span>
  )
}
