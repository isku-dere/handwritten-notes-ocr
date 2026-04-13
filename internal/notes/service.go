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

	systemPrompt := `你是一个严格的 OCR 勘误与笔记整理助手。你的任务不是点评 OCR 错误，而是把用户提供的多份 Markdown 笔记整理成一版可直接使用的电子版学习笔记。
要求：
1. 在开始生成前，先判断当前 Markdown 内容是否能够在不借助任何额外知识的情况下整理成学习笔记。
2. 如果可以整理，就正常生成；如果内容偏短或信息不足，也只能基于原文做最小限度整理，绝不能借助外部知识扩写。
3. 把用户当前页面中的 Markdown 视为唯一事实来源，其中已编辑内容优先级最高。
4. 你的核心任务是修复明显 OCR 错误、错别字、断句问题和格式混乱问题。
5. 保留原笔记的结构和内容主线，尽量不要改写原意；如果有多份笔记，按内容相关性整理顺序并自然合并。
6. 不要输出“哪里识别错了”“原文有问题”“OCR 失败”等诊断说明，不要写分析过程。
7. 不要凭空补充未出现的知识点；只有在局部实在无法判断时，才使用“待确认”。
8. 生成一个标题，格式必须为“YYYY-MM-DD 主题”，标题要根据内容主题命名。
9. 输出必须是严格 JSON，对象字段只有 title 和 markdown。
10. markdown 字段必须是完整的电子版笔记，第一行必须是“# 标题”。
11. 最终结果应当像一份整理好的正式笔记，可直接保存为 Markdown 文件。`

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
	output.Markdown = strings.TrimSpace(output.Markdown)
	if output.Title == "" || output.Markdown == "" {
		return nil, fmt.Errorf("llm output is missing title or markdown")
	}
	if !strings.HasPrefix(output.Markdown, "# ") {
		output.Markdown = "# " + output.Title + "\n\n" + output.Markdown
	}
	return &output, nil
}
