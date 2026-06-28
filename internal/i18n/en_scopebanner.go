// SPDX-License-Identifier: MIT

package i18n

// scopeBannerKeys are the member-scope banner strings (C281). Kept in a
// separate file from en.go so concurrent agent WIP doesn't cause merge
// conflicts — same init-merge pattern as other feature key files.
var scopeBannerKeys = Catalog{
	// Label format: "Viewing as <member name>"
	"scope.viewingAs": "Viewing as %s",
	// CTA to clear the active-member filter and return to the full household view.
	"scope.viewAll": "View all",
	// Accessible description of the "view all" action for screen readers.
	"scope.viewAllTitle": "Clear member filter and return to everyone's view",
	// aria-label for the banner itself.
	"scope.bannerLabel": "Member scope active",
}

func init() {
	for k, v := range scopeBannerKeys {
		english[k] = v
	}
}
