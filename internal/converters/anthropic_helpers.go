package converters

import (
	"encoding/json"
	"fmt"

	"ai_gateway/internal/models"
)

type normalizedAnthropicBlock struct {
	Type      string
	Text      string
	Source    map[string]interface{}
	ID        string
	Name      string
	Input     interface{}
	ToolUseID string
	Content   interface{}
	IsError   *bool
}

func extractSystemText(system interface{}) string {
	if system == nil {
		return ""
	}

	switch v := system.(type) {
	case string:
		return v
	case []models.SystemBlock:
		var text string
		for _, block := range v {
			if block.Type == "text" {
				text += block.Text
			}
		}
		return text
	case []models.ContentBlock:
		var text string
		for _, block := range v {
			if block.Type == "text" {
				text += block.Text
			}
		}
		return text
	case []map[string]interface{}:
		var text string
		for _, block := range v {
			if getString(block, "type") == "text" {
				text += getString(block, "text")
			}
		}
		return text
	case []interface{}:
		var text string
		for _, item := range v {
			switch block := item.(type) {
			case models.ContentBlock:
				if block.Type == "text" {
					text += block.Text
				}
			case map[string]interface{}:
				if getString(block, "type") == "text" {
					text += getString(block, "text")
				}
			}
		}
		return text
	default:
		return ""
	}
}

func normalizeAnthropicBlocks(content interface{}) []normalizedAnthropicBlock {
	switch v := content.(type) {
	case []models.ContentBlock:
		blocks := make([]normalizedAnthropicBlock, 0, len(v))
		for _, block := range v {
			blocks = append(blocks, normalizeBlockFromContentBlock(block))
		}
		return blocks
	case []interface{}:
		blocks := make([]normalizedAnthropicBlock, 0, len(v))
		for _, item := range v {
			block, ok := normalizeAnthropicBlock(item)
			if ok {
				blocks = append(blocks, block)
			}
		}
		return blocks
	case []map[string]interface{}:
		blocks := make([]normalizedAnthropicBlock, 0, len(v))
		for _, item := range v {
			blocks = append(blocks, normalizeBlockFromMap(item))
		}
		return blocks
	default:
		return nil
	}
}

func normalizeAnthropicBlock(raw interface{}) (normalizedAnthropicBlock, bool) {
	switch block := raw.(type) {
	case models.ContentBlock:
		return normalizeBlockFromContentBlock(block), true
	case map[string]interface{}:
		return normalizeBlockFromMap(block), true
	default:
		return normalizedAnthropicBlock{}, false
	}
}

func normalizeBlockFromContentBlock(block models.ContentBlock) normalizedAnthropicBlock {
	var source map[string]interface{}
	if block.Source != nil {
		source = map[string]interface{}{
			"type":       block.Source.Type,
			"media_type": block.Source.MediaType,
			"data":       block.Source.Data,
		}
	}

	var isError *bool
	if block.IsError {
		val := block.IsError
		isError = &val
	}

	return normalizedAnthropicBlock{
		Type:      block.Type,
		Text:      block.Text,
		Source:    source,
		ID:        block.ID,
		Name:      block.Name,
		Input:     block.Input,
		ToolUseID: block.ToolUseID,
		Content:   block.Content,
		IsError:   isError,
	}
}

func normalizeBlockFromMap(block map[string]interface{}) normalizedAnthropicBlock {
	var isError *bool
	if raw, ok := block["is_error"].(bool); ok {
		isError = &raw
	}

	return normalizedAnthropicBlock{
		Type:      getString(block, "type"),
		Text:      getString(block, "text"),
		Source:    mapValue(block, "source"),
		ID:        getString(block, "id"),
		Name:      getString(block, "name"),
		Input:     block["input"],
		ToolUseID: getString(block, "tool_use_id"),
		Content:   block["content"],
		IsError:   isError,
	}
}

func mapValue(m map[string]interface{}, key string) map[string]interface{} {
	if v, ok := m[key].(map[string]interface{}); ok {
		return v
	}
	return nil
}

func blockToolResultID(block normalizedAnthropicBlock) string {
	if block.ToolUseID != "" {
		return block.ToolUseID
	}
	return block.ID
}

func stringifyContent(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		return string(v)
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(bytes)
	}
}

func extractOpenAIContentParts(content interface{}) (string, []models.ContentBlock) {
	switch v := content.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case []models.ContentPart:
		var text string
		var blocks []models.ContentBlock
		for _, part := range v {
			switch part.Type {
			case "text":
				text += part.Text
			case "image_url":
				if part.ImageURL != nil && part.ImageURL.URL != "" {
					blocks = append(blocks, models.ContentBlock{
						Type: "image",
						Source: &models.ImageSource{
							Type: "base64",
							Data: part.ImageURL.URL,
						},
					})
				}
			}
		}
		return text, blocks
	case []interface{}:
		var text string
		var blocks []models.ContentBlock
		for _, item := range v {
			partMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			switch getString(partMap, "type") {
			case "text":
				text += getString(partMap, "text")
			case "image_url":
				if imageURL, ok := partMap["image_url"].(map[string]interface{}); ok {
					url := getString(imageURL, "url")
					if url != "" {
						blocks = append(blocks, models.ContentBlock{
							Type: "image",
							Source: &models.ImageSource{
								Type: "base64",
								Data: url,
							},
						})
					}
				}
			}
		}
		return text, blocks
	case []map[string]interface{}:
		var text string
		var blocks []models.ContentBlock
		for _, partMap := range v {
			switch getString(partMap, "type") {
			case "text":
				text += getString(partMap, "text")
			case "image_url":
				if imageURL, ok := partMap["image_url"].(map[string]interface{}); ok {
					url := getString(imageURL, "url")
					if url != "" {
						blocks = append(blocks, models.ContentBlock{
							Type: "image",
							Source: &models.ImageSource{
								Type: "base64",
								Data: url,
							},
						})
					}
				}
			}
		}
		return text, blocks
	default:
		return "", nil
	}
}
