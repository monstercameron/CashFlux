package theme

import (
	"fmt"
	"strings"
)

// ImageMaxBytes caps an uploaded banner image's raw file size. Banner images are
// embedded as data URLs in the durable store, so the cap protects that store
// (base64 inflates the bytes by ~33%). A reasonably compressed wide JPEG/WebP
// banner sits well under this.
const ImageMaxBytes = 2 << 20 // 2 MiB

// imageMIMEs is the set of accepted banner image formats.
var imageMIMEs = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
	"image/gif":  true,
}

// imageExtMIME maps an image file extension to a canonical MIME type, so the wasm
// layer can recover a type when the browser reports none.
var imageExtMIME = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
	".gif":  "image/gif",
}

// ValidImageMIME reports whether mime is an accepted banner image format.
func ValidImageMIME(mime string) bool {
	return imageMIMEs[strings.ToLower(strings.TrimSpace(mime))]
}

// ImageMIMEForName infers an image MIME type from a file name's extension, or ""
// if the extension isn't a known image type. Use it as a fallback when the
// browser's reported MIME type is empty.
func ImageMIMEForName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	for ext, mime := range imageExtMIME {
		if strings.HasSuffix(lower, ext) {
			return mime
		}
	}
	return ""
}

// ValidateImageUpload reports human-readable problems with a banner image upload
// — an unsupported format, an empty file, or one over the size cap — or nil if
// the upload is acceptable.
func ValidateImageUpload(mime string, size int) []string {
	var errs []string
	if !ValidImageMIME(mime) {
		errs = append(errs, "That file isn't a supported image format. Use PNG, JPEG, WebP, or GIF.")
	}
	if size <= 0 {
		errs = append(errs, "The image file is empty.")
	}
	if size > ImageMaxBytes {
		errs = append(errs, fmt.Sprintf("The image is too large (max %d MB).", ImageMaxBytes/(1<<20)))
	}
	return errs
}

// Banner kinds.
const (
	BannerNone     = "none"     // no banner band
	BannerGradient = "gradient" // a built-in CSS gradient
	BannerImage    = "image"    // an uploaded image (data URL)
)

// Banner is the optional decorative header band shown atop the dashboard. It is
// either nothing, one of the built-in gradients, or a user-uploaded image. The
// value is a CSS gradient expression (for gradients) or a data URL (for images);
// it is rendered purely decoratively (no essential text sits on it), so it can't
// hurt content legibility.
type Banner struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
	Name  string `json:"name"`
}

// None reports whether the banner is empty (no band shown).
func (b Banner) None() bool {
	return b.Kind == "" || b.Kind == BannerNone || strings.TrimSpace(b.Value) == ""
}

// CSS returns the value for the band's `background-image` property — the gradient
// expression as-is, or `url(<dataURL>)` for an image — or "" when there is no
// banner.
func (b Banner) CSS() string {
	switch b.Kind {
	case BannerGradient:
		if strings.TrimSpace(b.Value) == "" {
			return ""
		}
		return b.Value
	case BannerImage:
		if strings.TrimSpace(b.Value) == "" {
			return ""
		}
		return fmt.Sprintf("url(%s)", b.Value)
	default:
		return ""
	}
}

// bannerPresets are the built-in gradient banners, dark-friendly diagonal washes
// that read as a calm decorative band under the app's light text.
var bannerPresets = []Banner{
	{Kind: BannerGradient, Name: "Aurora", Value: "linear-gradient(135deg, #1f2c4d 0%, #3a1f4d 100%)"},
	{Kind: BannerGradient, Name: "Sunrise", Value: "linear-gradient(135deg, #4d3a1f 0%, #4d1f2c 100%)"},
	{Kind: BannerGradient, Name: "Forest", Value: "linear-gradient(135deg, #16332a 0%, #1f4d3a 100%)"},
	{Kind: BannerGradient, Name: "Slate", Value: "linear-gradient(135deg, #1c1f24 0%, #2a2f38 100%)"},
}

// BannerPresets returns the built-in gradient banners for a banner picker.
func BannerPresets() []Banner {
	out := make([]Banner, len(bannerPresets))
	copy(out, bannerPresets)
	return out
}

// ImageBanner builds a banner from an uploaded image's data URL and a label.
func ImageBanner(dataURL, name string) Banner {
	return Banner{Kind: BannerImage, Value: dataURL, Name: name}
}
