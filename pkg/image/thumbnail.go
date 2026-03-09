package image

import (
	"bytes"

	"github.com/disintegration/imaging"
)

// @func thumbnail
// @description 이미지를 200x200 정사각형으로 크롭하여 PNG 썸네일을 생성한다

type ThumbnailInput struct {
	Data []byte
}

type ThumbnailOutput struct {
	Data []byte
}

func Thumbnail(in ThumbnailInput) (ThumbnailOutput, error) {
	src, err := imaging.Decode(bytes.NewReader(in.Data))
	if err != nil {
		return ThumbnailOutput{}, err
	}
	thumb := imaging.Fill(src, 200, 200, imaging.Center, imaging.Lanczos)
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, thumb, imaging.PNG); err != nil {
		return ThumbnailOutput{}, err
	}
	return ThumbnailOutput{Data: buf.Bytes()}, nil
}
