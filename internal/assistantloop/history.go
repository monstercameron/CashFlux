// SPDX-License-Identifier: MIT

package assistantloop

import (
	"encoding/json"
	"errors"

	"github.com/monstercameron/CashFlux/internal/ai"
)

// ErrRequiredMessagesTooLarge means the fixed system context and current turn
// cannot fit within the configured relay limits even after dropping all history.
var ErrRequiredMessagesTooLarge = errors.New("assistant current request exceeds the history relay limit")

// ErrIncompleteToolProtocol means the messages contain a tool call without all
// of its matching results, or a tool result with no preceding call.
var ErrIncompleteToolProtocol = errors.New("assistant messages contain an incomplete tool exchange")

// ErrMessageTooLarge means one mandatory message exceeds the relay's per-message limit.
var ErrMessageTooLarge = errors.New("assistant message exceeds the per-message relay limit")

// BoundRequestHistory trims only prior conversation history. prefixCount messages
// at the start and every message from currentStart onward are mandatory; the latter
// contains the current user turn and any in-progress tool protocol. It retains the
// newest complete prior turns that fit both limits and returns an error rather than
// dropping part of the current turn or orphaning a tool call/result.
//
// maxMessages counts all returned messages. maxBytes counts the encoded messages
// array plus any Responses API reasoning payloads, which are not part of ai.Message JSON.
func BoundRequestHistory(messages []ai.Message, prefixCount, currentStart, maxMessages, maxBytes, maxMessageBytes int) ([]ai.Message, error) {
	if prefixCount < 0 || currentStart < prefixCount || currentStart > len(messages) || maxMessages <= 0 || maxBytes <= 0 || maxMessageBytes <= 0 {
		return nil, ErrRequiredMessagesTooLarge
	}
	prefix := messages[:prefixCount]
	prior := messages[prefixCount:currentStart]
	current := messages[currentStart:]
	if err := validateToolProtocol(prior); err != nil {
		return nil, err
	}
	if err := validateToolProtocol(current); err != nil {
		return nil, err
	}

	count := len(prefix) + len(current)
	bytesUsed, err := messageBytes(prefix)
	if err != nil {
		return nil, err
	}
	currentBytes, err := messageBytes(current)
	if err != nil {
		return nil, err
	}
	bytesUsed += currentBytes
	if hasOversizedMessage(prefix, maxMessageBytes) || hasOversizedMessage(current, maxMessageBytes) {
		return nil, ErrMessageTooLarge
	}
	if count > maxMessages || requestBytes(bytesUsed, count) > maxBytes {
		return nil, ErrRequiredMessagesTooLarge
	}

	bundles, err := completeTurnBundles(prior)
	if err != nil {
		return nil, err
	}
	selected := make([][]ai.Message, 0, len(bundles))
	for i := len(bundles) - 1; i >= 0; i-- {
		bundle := bundles[i]
		bundleBytes, err := messageBytes(bundle)
		if err != nil {
			return nil, err
		}
		if hasOversizedMessage(bundle, maxMessageBytes) {
			continue
		}
		if count+len(bundle) > maxMessages || requestBytes(bytesUsed+bundleBytes, count+len(bundle)) > maxBytes {
			break
		}
		selected = append(selected, bundle)
		count += len(bundle)
		bytesUsed += bundleBytes
	}

	result := make([]ai.Message, 0, count)
	result = append(result, prefix...)
	for i := len(selected) - 1; i >= 0; i-- {
		result = append(result, selected[i]...)
	}
	result = append(result, current...)
	return result, nil
}

func hasOversizedMessage(messages []ai.Message, maxBytes int) bool {
	for _, message := range messages {
		bytes, err := messageBytes([]ai.Message{message})
		if err != nil || bytes > maxBytes {
			return true
		}
	}
	return false
}

func requestBytes(messagePayloadBytes, count int) int {
	if count == 0 {
		return 2 // []
	}
	return messagePayloadBytes + count + 1 // brackets plus commas
}

func completeTurnBundles(messages []ai.Message) ([][]ai.Message, error) {
	bundles := make([][]ai.Message, 0, len(messages))
	for start := 0; start < len(messages); {
		end := start + 1
		for end < len(messages) && messages[end].Role != ai.RoleUser {
			end++
		}
		bundle := messages[start:end]
		if err := validateToolProtocol(bundle); err != nil {
			return nil, err
		}
		bundles = append(bundles, bundle)
		start = end
	}
	return bundles, nil
}

func validateToolProtocol(messages []ai.Message) error {
	pending := map[string]struct{}{}
	for _, message := range messages {
		if len(pending) > 0 {
			if message.Role != ai.RoleTool {
				return ErrIncompleteToolProtocol
			}
			if _, ok := pending[message.ToolCallID]; !ok || message.ToolCallID == "" {
				return ErrIncompleteToolProtocol
			}
			delete(pending, message.ToolCallID)
			continue
		}
		if message.Role == ai.RoleTool {
			return ErrIncompleteToolProtocol
		}
		if message.Role != ai.RoleAssistant || len(message.ToolCalls) == 0 {
			continue
		}
		for _, call := range message.ToolCalls {
			if call.ID == "" {
				return ErrIncompleteToolProtocol
			}
			if _, duplicate := pending[call.ID]; duplicate {
				return ErrIncompleteToolProtocol
			}
			pending[call.ID] = struct{}{}
		}
	}
	if len(pending) != 0 {
		return ErrIncompleteToolProtocol
	}
	return nil
}

func messageBytes(messages []ai.Message) (int, error) {
	total := 0
	for _, message := range messages {
		encoded, err := json.Marshal(message)
		if err != nil {
			return 0, err
		}
		total += len(encoded)
		for _, reasoning := range message.ReasoningRaw {
			total += len(reasoning)
		}
	}
	return total, nil
}
