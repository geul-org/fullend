package text

import "github.com/gosimple/slug"

// @func generateSlug
// @description 텍스트를 URL-safe slug로 변환한다

type GenerateSlugRequest struct {
	Text string
}

type GenerateSlugResponse struct {
	Slug string
}

func GenerateSlug(req GenerateSlugRequest) (GenerateSlugResponse, error) {
	return GenerateSlugResponse{Slug: slug.Make(req.Text)}, nil
}
