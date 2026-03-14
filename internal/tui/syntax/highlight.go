// Package syntax provides syntax highlighting for diff views using Chroma.
//
// The highlighter tokenizes the entire diff source as a single block so that
// multi-line constructs (block comments, multi-line strings) are handled
// correctly. Tokens are then split back into per-line groups and cached.
// During rendering, tab expansion and horizontal slicing are applied on the
// token stream directly, preserving per-token colors across the viewport.
package syntax

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	chromaStyles "github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

// styledToken is a single syntax-colored text fragment.
type styledToken struct {
	Text string
	// FG is the foreground color as hex (#RRGGBB), or empty for default.
	FG string
	// Bold / Italic flags.
	Bold   bool
	Italic bool
}

// Highlighter tokenizes source lines and renders them with syntax colors.
type Highlighter struct {
	style *chroma.Style

	// lineTokens[i] holds the tokens for source line i.
	lineTokens [][]styledToken
}

// New creates a Highlighter for the given file path and chroma style name.
// diffLines are the raw diff lines (including +/-/space prefix).
// The highlighter strips diff prefixes, joins lines into a single source
// block, tokenizes with the appropriate language lexer, and splits the
// tokens back into per-line groups.
// Returns nil if no lexer is found for the file type.
func New(filePath, styleName string, diffLines []string) *Highlighter {
	if filePath == "" || len(diffLines) == 0 {
		return nil
	}

	// Detect lexer by filename/extension.
	lexer := lexers.Match(filepath.Base(filePath))
	if lexer == nil {
		ext := filepath.Ext(filePath)
		if ext != "" {
			lexer = lexers.Match("file" + ext)
		}
	}
	if lexer == nil {
		return nil
	}
	lexer = chroma.Coalesce(lexer)

	// Resolve chroma style.
	style := chromaStyles.Get(styleName)
	if style == nil {
		style = chromaStyles.Get("catppuccin-mocha")
	}
	if style == nil {
		style = chromaStyles.Fallback
	}

	// Build the source block: strip diff prefixes and join with newlines.
	// Keep a mapping of diff-line-index → source-line-index.
	// Non-hunk lines (headers, metadata) are mapped to nil token slices.
	sourceLines := make([]string, len(diffLines))
	for i, line := range diffLines {
		if len(line) > 0 {
			ch := line[0]
			if ch == '+' || ch == '-' || ch == ' ' {
				sourceLines[i] = line[1:]
				continue
			}
		}
		// Hunk headers, diff headers, etc. — keep as-is for line count
		// but they won't get meaningful syntax tokens.
		sourceLines[i] = line
	}

	source := strings.Join(sourceLines, "\n")

	// Tokenize the entire source block.
	iter, err := lexer.Tokenise(nil, source)
	if err != nil {
		return nil
	}

	// Split tokens into per-line groups by scanning for newlines within tokens.
	allLineTokens := make([][]styledToken, len(diffLines))
	lineIdx := 0

	for _, tok := range iter.Tokens() {
		if tok.Value == "" {
			continue
		}
		entry := style.Get(tok.Type)
		fg := ""
		if entry.Colour.IsSet() {
			fg = entry.Colour.String()
		}
		bold := entry.Bold == chroma.Yes
		italic := entry.Italic == chroma.Yes

		// A single chroma token may span multiple lines (e.g. multi-line
		// strings or comments). Split on newlines and distribute.
		parts := strings.Split(tok.Value, "\n")
		for pi, part := range parts {
			if pi > 0 {
				lineIdx++
				if lineIdx >= len(diffLines) {
					break
				}
			}
			if lineIdx >= len(diffLines) {
				break
			}
			if part == "" {
				continue
			}
			allLineTokens[lineIdx] = append(allLineTokens[lineIdx], styledToken{
				Text:   part,
				FG:     fg,
				Bold:   bold,
				Italic: italic,
			})
		}
	}

	return &Highlighter{
		style:      style,
		lineTokens: allLineTokens,
	}
}

// RenderLine renders a single diff line with syntax highlighting.
//
// Parameters:
//   - lineIdx: index into the original diffLines array
//   - lineType: the diff prefix byte (+, -, ' ', @)
//   - bg: background color for the diff line type
//   - defaultFG: fallback foreground when token has no syntax color
//   - scrollX: horizontal scroll offset (characters to skip)
//   - width: total visible width to render (including prefix character)
//   - diffPrefixStyle: lipgloss style for the prefix character (+/-/space)
func (h *Highlighter) RenderLine(
	lineIdx int,
	lineType byte,
	bg, defaultFG lipgloss.Color,
	scrollX, width int,
	diffPrefixStyle lipgloss.Style,
) string {
	// Reserve 1 column for the diff prefix character.
	contentWidth := width - 1
	if contentWidth < 1 {
		contentWidth = 1
	}

	// Get the prefix character.
	prefix := " "
	switch lineType {
	case '+':
		prefix = "+"
	case '-':
		prefix = "-"
	}
	prefixStr := diffPrefixStyle.Render(prefix)

	// Get tokens for this line.
	var tokens []styledToken
	if lineIdx >= 0 && lineIdx < len(h.lineTokens) {
		tokens = h.lineTokens[lineIdx]
	}
	if len(tokens) == 0 {
		// No tokens — render empty line with background.
		pad := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", contentWidth))
		return prefixStr + pad
	}

	// Walk the token stream, expanding tabs and applying horizontal slicing.
	var sb strings.Builder
	col := 0     // current visual column (after tab expansion)
	emitted := 0 // visible characters emitted to output

	for _, tok := range tokens {
		if emitted >= contentWidth {
			break
		}

		s := lipgloss.NewStyle().Background(bg)
		if tok.FG != "" {
			s = s.Foreground(lipgloss.Color(tok.FG))
		} else {
			s = s.Foreground(defaultFG)
		}
		if tok.Bold {
			s = s.Bold(true)
		}
		if tok.Italic {
			s = s.Italic(true)
		}

		// Process each character in the token for tab expansion and slicing.
		var chunk strings.Builder
		for _, r := range tok.Text {
			if emitted >= contentWidth {
				break
			}

			if r == '\t' {
				// Tab expansion: fill to next 4-column boundary.
				spaces := 4 - (col % 4)
				for i := 0; i < spaces; i++ {
					col++
					if col <= scrollX {
						continue
					}
					if emitted >= contentWidth {
						break
					}
					chunk.WriteByte(' ')
					emitted++
				}
			} else {
				col++
				if col <= scrollX {
					continue
				}
				if emitted >= contentWidth {
					break
				}
				chunk.WriteRune(r)
				emitted++
			}
		}

		if chunk.Len() > 0 {
			sb.WriteString(s.Render(chunk.String()))
		}
	}

	// Pad remaining space.
	if emitted < contentWidth {
		pad := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", contentWidth-emitted))
		sb.WriteString(pad)
	}

	return prefixStr + sb.String()
}

// LineCount returns the number of lines the highlighter has tokens for.
func (h *Highlighter) LineCount() int {
	return len(h.lineTokens)
}
