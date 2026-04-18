package notes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"handwritten-notes-ocr/internal/llm"
)

type InputNote struct {
	FileName string `json:"fileName"`
	Markdown string `json:"markdown"`
	Edited   bool   `json:"edited,omitempty"`
}

type OutputNote struct {
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
}

type PrecheckResult struct {
	ShouldConfirm bool   `json:"shouldConfirm"`
	Message       string `json:"message,omitempty"`
}

type Service struct {
	LLM *llm.Client
}

func (s *Service) Precheck(notes []InputNote) PrecheckResult {
	filtered := filterNotes(notes)
	if len(filtered) == 0 {
		return PrecheckResult{
			ShouldConfirm: true,
			Message:       "当前没有可用于生成学习笔记的 Markdown 内容。",
		}
	}

	totalChars := 0
	meaningfulLines := 0
	for _, note := range filtered {
		for _, line := range strings.Split(note.Markdown, "\n") {
			trimmed := strings.TrimSpace(strings.TrimLeft(line, "#-*0123456789.> `\t"))
			if trimmed == "" {
				continue
			}
			meaningfulLines++
			totalChars += len([]rune(trimmed))
		}
	}

	if totalChars < 120 || meaningfulLines < 3 {
		return PrecheckResult{
			ShouldConfirm: true,
			Message:       "当前内容较短，可能不足以在不借助额外知识的情况下形成学习笔记。继续生成可能增加额外 token 消耗，是否继续？",
		}
	}

	return PrecheckResult{}
}

func (s *Service) GenerateStudyNote(ctx context.Context, notes []InputNote) (*OutputNote, error) {
	if s.LLM == nil {
		return nil, fmt.Errorf("LLM service is not configured")
	}
	systemPrompt, userPrompt, err := buildPrompts(notes)
	if err != nil {
		return nil, err
	}

	content, err := s.LLM.Chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	result, err := parseOutput(content)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) GenerateStudyNoteStream(ctx context.Context, notes []InputNote, onDelta func(string) error) (*OutputNote, error) {
	if s.LLM == nil {
		return nil, fmt.Errorf("LLM service is not configured")
	}
	systemPrompt, userPrompt, err := buildPrompts(notes)
	if err != nil {
		return nil, err
	}

	content, err := s.LLM.ChatStream(ctx, systemPrompt, userPrompt, onDelta)
	if err != nil {
		return nil, err
	}

	result, err := parseOutput(content)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func buildPrompts(notes []InputNote) (string, string, error) {
	filtered := filterNotes(notes)
	if len(filtered) == 0 {
		return "", "", fmt.Errorf("no markdown notes available for summarization")
	}

	systemPrompt := `你是一个“手写笔记电子化排版助手”，不是 OCR 错误分析助手。

你的唯一目标：
把用户提供的 Markdown 草稿整理成一份可直接使用的电子版学习笔记。

最高优先级硬性约束：
1. 最终 markdown 只能包含整理后的笔记正文。
2. 绝对禁止输出任何 OCR 纠错说明、识别错误分析、修改说明、处理过程、勘误报告、置信度说明。
3. 绝对禁止出现类似这些表达：
   - “OCR 识别”
   - “识别错误”
   - “原文可能是”
   - “已修正”
   - “纠错”
   - “勘误”
   - “根据上下文推测”
   - “无法判断”
   - “以下是修正后的”
   - “整理说明”
4. 如果某处无法确定，只在笔记对应位置保留“待确认”，不要解释为什么待确认。
5. 不要新增输入中没有出现的知识点，不要为了让笔记更完整而扩写外部知识。

内容处理规则：
1. 把用户当前页面中的 Markdown 视为唯一事实来源，其中 edited=true 的内容优先级最高。
2. 修复明显错别字、断句、格式混乱和 Markdown 层级问题，但不要解释修复动作。
3. 尽量保留原笔记结构、标题层级、列表关系和内容主线。
4. 如果有多份笔记，按内容相关性和学习顺序自然合并。
5. 在开始生成前，先判断当前 Markdown 是否能在不借助额外知识的情况下形成笔记；如果内容偏短，只做最小限度整理。

输出格式：
1. 只输出严格 JSON，不要输出 Markdown 代码块。
2. JSON 对象字段只能有 title 和 markdown。
3. title 格式必须是“YYYY-MM-DD 主题”。
4. markdown 字段必须是一份完整的电子版笔记，第一行必须是“# 标题”。
5. markdown 中不要出现“OCR”“识别”“纠错”“勘误”“修正说明”等元信息。`

	userPayload, err := json.Marshal(map[string]any{
		"current_date": time.Now().Format("2006-01-02"),
		"notes":        filtered,
	})
	if err != nil {
		return "", "", fmt.Errorf("marshal summarization input: %w", err)
	}
	return systemPrompt, string(userPayload), nil
}

func filterNotes(notes []InputNote) []InputNote {
	filtered := make([]InputNote, 0, len(notes))
	for _, note := range notes {
		if strings.TrimSpace(note.Markdown) == "" {
			continue
		}
		filtered = append(filtered, note)
	}
	return filtered
}

func parseOutput(content string) (*OutputNote, error) {
	candidate := strings.TrimSpace(content)
	candidate = strings.TrimPrefix(candidate, "```json")
	candidate = strings.TrimPrefix(candidate, "```")
	candidate = strings.TrimSuffix(candidate, "```")
	candidate = strings.TrimSpace(candidate)

	var output OutputNote
	if err := json.Unmarshal([]byte(candidate), &output); err != nil {
		start := strings.Index(candidate, "{")
		end := strings.LastIndex(candidate, "}")
		if start >= 0 && end > start {
			if retryErr := json.Unmarshal([]byte(candidate[start:end+1]), &output); retryErr != nil {
				return nil, fmt.Errorf("decode llm note output: %w", err)
			}
		} else {
			return nil, fmt.Errorf("decode llm note output: %w", err)
		}
	}

	output.Title = strings.TrimSpace(output.Title)
	output.Markdown = sanitizeStudyNoteMarkdown(output.Markdown)
	if output.Title == "" || output.Markdown == "" {
		return nil, fmt.Errorf("llm output is missing title or markdown")
	}
	if !strings.HasPrefix(output.Markdown, "# ") {
		output.Markdown = "# " + output.Title + "\n\n" + output.Markdown
	}
	return &output, nil
}

func sanitizeStudyNoteMarkdown(markdown string) string {
	lines := strings.Split(strings.TrimSpace(markdown), "\n")
	cleaned := make([]string, 0, len(lines))
	skipSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isMetaHeading(trimmed) {
			skipSection = true
			continue
		}
		if skipSection && strings.HasPrefix(trimmed, "#") {
			skipSection = false
		}
		if skipSection {
			continue
		}
		if isMetaExplanationLine(trimmed) {
			continue
		}
		cleaned = append(cleaned, line)
	}

	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func isMetaHeading(line string) bool {
	if !strings.HasPrefix(line, "#") {
		return false
	}
	return containsAny(line, []string{
		"OCR", "识别", "纠错", "勘误", "修正说明", "整理说明", "处理说明", "错误分析",
	})
}

func isMetaExplanationLine(line string) bool {
	return containsAny(line, []string{
		"OCR识别", "OCR 识别", "识别错误", "原文可能", "已修正", "纠错", "勘误",
		"根据上下文推测", "以下是修正", "以下为修正", "整理说明", "处理过程",
	})
}

func containsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}
