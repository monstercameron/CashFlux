// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

func (c *documentsController) removeSourceFile(source string) {
	if c.busy() || c.aiLoading.Get() {
		return
	}
	c.cancelAI()
	if source == "pdf" {
		c.pdfData.Set("")
		c.pdfBytes.Set(nil)
		c.pdfName.Set("")
		c.pdfSaveCopy.Set(false)
	} else {
		c.imageURL.Set("")
		c.imageName.Set("")
		c.imageSize.Set(0)
		c.imageDraftAtom.Set("")
	}
	c.aiErr.Set("")
}
