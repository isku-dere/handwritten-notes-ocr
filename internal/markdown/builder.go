package markdown

import (
	"fmt"
	"regexp"
	"strings"

	"handwritten-notes-ocr/internal/ocr"
)

var headingPattern = regexp.MustCompile(`^[0-9一二三四五六七八九十]+[\.、\)]`)

func Build(lines []ocr.Line) string {
	if len(lines) == 0 {
		return "# 未识别到内容\n"
	}

	var b strings.Builder
	titleWritten := false

	for i, line := range lines {
		text := normalize(line.Text)
		if text == "" {
			continue
		}

		switch {
		case !titleWritten && looksLikeTitle(text):
			b.WriteString("# " + text + "\n\n")
			titleWritten = true
		case looksLikeHeading(text):
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n\n") {
				b.WriteString("\n")
			}
			b.WriteString("## " + trimHeadingPrefix(text) + "\n\n")
		case looksLikeListItem(text):
			b.WriteString("- " + trimListPrefix(text) + "\n")
		case looksLikeChecklist(text):
			b.WriteString("- [ ] " + trimChecklistPrefix(text) + "\n")
		default:
			if i > 0 && previousNeedsBlankLine(lines[i-1].Text, text) && !strings.HasSuffix(b.String(), "\n\n") {
				b.WriteString("\n")
			}
			b.WriteString(text + "\n\n")
		}
	}

	output := strings.TrimSpace(b.String())
	if output == "" {
		return "# 未识别到内容\n"
	}

	return output + "\n"
}

func normalize(text string) string {
	replacer := strings.NewReplacer("：", ":", "（", "(", "）", ")", "\t", " ")
	text = replacer.Replace(text)
	text = strings.TrimSpace(text)
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return text
}

func looksLikeTitle(text string) bool {
	return len([]rune(text)) <= 24 && !strings.Contains(text, ":") && !looksLikeListItem(text)
}

func looksLikeHeading(text string) bool {
	return strings.HasSuffix(text, ":") || headingPattern.MatchString(text)
}

func trimHeadingPrefix(text string) string {
	text = strings.TrimSuffix(text, ":")
	return headingPattern.ReplaceAllString(text, "")
}

func looksLikeListItem(text string) bool {
	prefixes := []string{"- ", "* ", "• ", "· ", "1.", "2.", "3.", "4.", "5."}
	for _, prefix := range prefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func trimListPrefix(text string) string {
	for _, prefix := range []string{"- ", "* ", "• ", "· "} {
		if strings.HasPrefix(text, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(text, prefix))
		}
	}
	if matched := regexp.MustCompile(`^[0-9]+\.\s*`).FindString(text); matched != "" {
		return strings.TrimSpace(strings.TrimPrefix(text, matched))
	}
	return text
}

func looksLikeChecklist(text string) bool {
	return strings.HasPrefix(text, "[ ]") || strings.HasPrefix(text, "[]") || strings.HasPrefix(text, "口 ")
}

func trimChecklistPrefix(text string) string {
	for _, prefix := range []string{"[ ]", "[]", "口 "} {
		if strings.HasPrefix(text, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(text, prefix))
		}
	}
	return text
}

func previousNeedsBlankLine(prev, current string) bool {
	prev = normalize(prev)
	current = normalize(current)
	return len(prev) > 0 && len(current) > 0 && !looksLikeListItem(prev) && !looksLikeListItem(current)
}

func Debug(lines []ocr.Line) string {
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		parts = append(parts, fmt.Sprintf("%.2f %s", line.Score, line.Text))
	}
	return strings.Join(parts, "\n")
}
