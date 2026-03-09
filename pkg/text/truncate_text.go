package text

// @func truncateText
// @description 유니코드 안전하게 텍스트를 자른다

type TruncateTextRequest struct {
	Text      string
	MaxLength int
	Suffix    string // 말줄임 (기본 "...")
}

type TruncateTextResponse struct {
	Truncated string
}

func TruncateText(req TruncateTextRequest) (TruncateTextResponse, error) {
	suffix := req.Suffix
	if suffix == "" {
		suffix = "..."
	}
	runes := []rune(req.Text)
	if len(runes) <= req.MaxLength {
		return TruncateTextResponse{Truncated: req.Text}, nil
	}
	return TruncateTextResponse{Truncated: string(runes[:req.MaxLength]) + suffix}, nil
}
