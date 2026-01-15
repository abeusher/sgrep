package conv

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	// MaxTurnTokens is the maximum tokens per turn before splitting.
	// Approximately 4 characters per token.
	MaxTurnTokens = 1200

	// MaxTurnChars is the character limit derived from token limit.
	MaxTurnChars = MaxTurnTokens * 4

	// OverlapChars is the overlap for context continuity when splitting.
	OverlapChars = 200
)

// Chunker handles turn-based chunking of conversations.
type Chunker struct {
	maxChars    int
	overlapChars int
}

// NewChunker creates a new chunker with default settings.
func NewChunker() *Chunker {
	return &Chunker{
		maxChars:    MaxTurnChars,
		overlapChars: OverlapChars,
	}
}

// ChunkerConfig allows customizing chunker behavior.
type ChunkerConfig struct {
	MaxTokens    int
	OverlapChars int
}

// NewChunkerWithConfig creates a chunker with custom configuration.
func NewChunkerWithConfig(cfg ChunkerConfig) *Chunker {
	maxChars := cfg.MaxTokens * 4
	if maxChars == 0 {
		maxChars = MaxTurnChars
	}
	overlapChars := cfg.OverlapChars
	if overlapChars == 0 {
		overlapChars = OverlapChars
	}
	return &Chunker{
		maxChars:    maxChars,
		overlapChars: overlapChars,
	}
}

// TurnChunk represents a chunk derived from a turn.
type TurnChunk struct {
	ID           string // session_id:turn_index[:chunk_index]
	SessionID    string
	TurnIndex    int
	ChunkIndex   int    // 0 for single chunk, 1+ for split chunks
	Content      string // Combined user + assistant content
	UserContent  string // Original user content
	AssistContent string // Original assistant content (may be partial if split)
}

// ChunkTurn converts a turn into one or more chunks.
// Most turns fit in a single chunk; long turns are split at paragraph boundaries.
func (c *Chunker) ChunkTurn(sessionID string, turn *Turn) []TurnChunk {
	// Create combined content for embedding
	combinedContent := c.formatTurnContent(turn)

	// If it fits, return single chunk
	if utf8.RuneCountInString(combinedContent) <= c.maxChars {
		return []TurnChunk{{
			ID:            fmt.Sprintf("%s:%d", sessionID, turn.Index),
			SessionID:     sessionID,
			TurnIndex:     turn.Index,
			ChunkIndex:    0,
			Content:       combinedContent,
			UserContent:   turn.UserContent,
			AssistContent: turn.AssistContent,
		}}
	}

	// Need to split - split the assistant content (usually the long part)
	return c.splitTurn(sessionID, turn)
}

// ChunkSession processes an entire session into chunks.
func (c *Chunker) ChunkSession(session *Session) []TurnChunk {
	var chunks []TurnChunk
	for i := range session.Turns {
		turnChunks := c.ChunkTurn(session.ID, &session.Turns[i])
		chunks = append(chunks, turnChunks...)
	}
	return chunks
}

// formatTurnContent creates the combined content for embedding.
func (c *Chunker) formatTurnContent(turn *Turn) string {
	var sb strings.Builder
	sb.WriteString("USER: ")
	sb.WriteString(strings.TrimSpace(turn.UserContent))
	sb.WriteString("\n\nASSISTANT: ")
	sb.WriteString(strings.TrimSpace(turn.AssistContent))
	return sb.String()
}

// splitTurn splits a long turn into multiple chunks.
func (c *Chunker) splitTurn(sessionID string, turn *Turn) []TurnChunk {
	var chunks []TurnChunk

	// User content is usually short, keep it intact
	userPart := "USER: " + strings.TrimSpace(turn.UserContent) + "\n\n"
	userLen := utf8.RuneCountInString(userPart)

	// Calculate available space for assistant content per chunk
	availableChars := c.maxChars - userLen - len("ASSISTANT: ")

	// Split assistant content at paragraph boundaries
	assistParts := c.splitAtParagraphs(turn.AssistContent, availableChars)

	for i, part := range assistParts {
		chunkID := fmt.Sprintf("%s:%d:%d", sessionID, turn.Index, i)

		var content strings.Builder
		content.WriteString(userPart)
		content.WriteString("ASSISTANT: ")
		content.WriteString(strings.TrimSpace(part))

		chunks = append(chunks, TurnChunk{
			ID:            chunkID,
			SessionID:     sessionID,
			TurnIndex:     turn.Index,
			ChunkIndex:    i,
			Content:       content.String(),
			UserContent:   turn.UserContent,
			AssistContent: part,
		})
	}

	return chunks
}

// splitAtParagraphs splits text at paragraph boundaries (double newlines).
func (c *Chunker) splitAtParagraphs(text string, maxChars int) []string {
	if maxChars <= 0 {
		maxChars = c.maxChars
	}

	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) <= maxChars {
		return []string{text}
	}

	var parts []string
	paragraphs := strings.Split(text, "\n\n")

	var current strings.Builder
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		paraLen := utf8.RuneCountInString(para)
		currentLen := utf8.RuneCountInString(current.String())

		// If adding this paragraph would exceed limit
		if currentLen > 0 && currentLen + paraLen + 2 > maxChars {
			// Save current and start new
			parts = append(parts, current.String())
			current.Reset()

			// Add overlap from end of previous content
			if c.overlapChars > 0 && len(parts) > 0 {
				prev := parts[len(parts)-1]
				runes := []rune(prev)
				if len(runes) > c.overlapChars {
					overlap := string(runes[len(runes)-c.overlapChars:])
					// Find last complete sentence or line
					if idx := strings.LastIndex(overlap, ". "); idx > 0 {
						overlap = overlap[idx+2:]
					} else if idx := strings.LastIndex(overlap, "\n"); idx > 0 {
						overlap = overlap[idx+1:]
					}
					current.WriteString(overlap)
					current.WriteString("\n\n")
				}
			}
		}

		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(para)
	}

	// Don't forget the last part
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	// Handle case where a single paragraph is too long
	var finalParts []string
	for _, part := range parts {
		if utf8.RuneCountInString(part) > maxChars {
			// Split by sentences or hard break
			subParts := c.hardSplit(part, maxChars)
			finalParts = append(finalParts, subParts...)
		} else {
			finalParts = append(finalParts, part)
		}
	}

	return finalParts
}

// hardSplit splits text when no good boundaries exist.
func (c *Chunker) hardSplit(text string, maxChars int) []string {
	var parts []string
	runes := []rune(text)

	for len(runes) > 0 {
		end := maxChars
		if end > len(runes) {
			end = len(runes)
		}

		// Try to find a sentence boundary
		if end < len(runes) {
			chunk := string(runes[:end])
			// Look for sentence end (. ! ?)
			for _, delim := range []string{". ", "! ", "? ", ".\n", "!\n", "?\n"} {
				if idx := strings.LastIndex(chunk, delim); idx >= 0 {
					idxRunes := utf8.RuneCountInString(chunk[:idx])
					if idxRunes > end/2 {
						end = idxRunes + utf8.RuneCountInString(delim)
						break
					}
				}
			}
			// Fall back to word boundary
			if end == maxChars {
				if idx := strings.LastIndex(chunk, " "); idx >= 0 {
					idxRunes := utf8.RuneCountInString(chunk[:idx])
					if idxRunes > end/2 {
						end = idxRunes
					}
				}
			}
		}

		if end > len(runes) {
			end = len(runes)
		}

		parts = append(parts, strings.TrimSpace(string(runes[:end])))
		runes = runes[end:]

		// Add overlap for continuity
		if len(runes) > 0 && c.overlapChars > 0 && len(parts) > 0 {
			prev := []rune(parts[len(parts)-1])
			if len(prev) > c.overlapChars {
				overlap := prev[len(prev)-c.overlapChars:]
				runes = append(overlap, runes...)
			}
		}
	}

	return parts
}

// EstimateTokens estimates the token count for text.
// Uses a simple heuristic of ~4 characters per token.
func EstimateTokens(text string) int {
	return (utf8.RuneCountInString(text) + 3) / 4
}

// EstimateTurnTokens estimates tokens for a turn.
func EstimateTurnTokens(turn *Turn) int {
	return EstimateTokens(turn.UserContent) + EstimateTokens(turn.AssistContent)
}

// EstimateSessionTokens estimates total tokens for a session.
func EstimateSessionTokens(session *Session) int {
	total := 0
	for _, turn := range session.Turns {
		total += EstimateTurnTokens(&turn)
	}
	return total
}
