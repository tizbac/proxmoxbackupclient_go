package machinebackuplib

import "strings"

// joinModel puts vendor and product together, leaving out a vendor that is
// empty, already part of the product, or a placeholder.
func joinModel(vendor, product string) string {
	vendor, product = strings.TrimSpace(vendor), strings.TrimSpace(product)
	switch strings.ToUpper(vendor) {
	case "", "ATA", "NVME", "(STANDARD DISK DRIVES)":
		return product
	}
	if product == "" {
		return vendor
	}
	if strings.HasPrefix(strings.ToUpper(product), strings.ToUpper(vendor)) {
		return product
	}
	return vendor + " " + product
}
