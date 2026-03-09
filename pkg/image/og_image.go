package image

import (
	"bytes"

	"github.com/disintegration/imaging"
)

// @func ogImage
// @description 이미지를 OG 이미지 규격(1200x630)으로 크롭하여 PNG로 출력한다

type OgImageInput struct {
	Data []byte
}

type OgImageOutput struct {
	Data []byte
}

func OgImage(in OgImageInput) (OgImageOutput, error) {
	src, err := imaging.Decode(bytes.NewReader(in.Data))
	if err != nil {
		return OgImageOutput{}, err
	}
	og := imaging.Fill(src, 1200, 630, imaging.Center, imaging.Lanczos)
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, og, imaging.PNG); err != nil {
		return OgImageOutput{}, err
	}
	return OgImageOutput{Data: buf.Bytes()}, nil
}
