package gravatar

import (
	"bytes"
	"crypto/sha256"
	"image"
	"io"
	"net/http"

	. "yo/ctx"
	. "yo/util"
	"yo/util/str"
)

func sha256HexOf(s string) (hex string, err error) {
	sha256 := sha256.New()
	_, err = sha256.Write([]byte(str.Lo(s)))
	hex = str.Fmt("%x", sha256.Sum(nil))
	return
}

func ImageUrlByEmailAddr(emailAddr string) (string, error) {
	sha256_hex, err := sha256HexOf(emailAddr)
	return "https://gravatar.com/avatar/" + sha256_hex + "?d=404", err
}
func ImageByEmailAddr(ctx *Ctx, emailAddr string) ([]byte, bool, error) {
	url_gravatar, err := ImageUrlByEmailAddr(emailAddr)
	if err != nil {
		return nil, false, err
	} else if http_req, err := http.NewRequestWithContext(ctx, "GET", url_gravatar, nil); (err != nil) || (http_req == nil) {
		return nil, false, err
	} else if resp, err := http.DefaultClient.Do(http_req); (err != nil) || (resp == nil) || (resp.Body == nil) {
		return nil, false, err
	} else {
		defer resp.Body.Close()
		var buf bytes.Buffer
		if _, err = io.Copy(&buf, resp.Body); (err != nil) || (buf.Len() == 0) {
			return nil, false, err
		} else {
			src_img := buf.Bytes() // must capture before Decode
			img, _, err := image.Decode(&buf)
			return If(img == nil, nil, src_img), true, err
		}
	}
}
