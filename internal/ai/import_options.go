// SPDX-License-Identifier: MIT

package ai

// DefaultStructuredMaxCompletionTokens caps import extraction replies. Import
// responses are bounded because source files can contain thousands of rows.
const DefaultStructuredMaxCompletionTokens = 4096
