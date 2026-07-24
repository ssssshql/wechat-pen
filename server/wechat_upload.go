package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/net/html"
)

const (
	wechatUploadImgURL    = "https://api.weixin.qq.com/cgi-bin/media/uploadimg"
	wechatAddMaterialURL  = "https://api.weixin.qq.com/cgi-bin/material/add_material"
)

const maxImageBytes = 1 << 20 // 1 MB
const maxImageDim = 1920
const defaultJPEGQuality = 75

type uploadImgResp struct {
	URL     string `json:"url"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type addMaterialResp struct {
	MediaID string `json:"media_id"`
	URL     string `json:"url"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// processContentImages downloads all <img> sources, uploads to WeChat, and replaces src URLs.
// If useUploadImg is true, uses the uploadimg API (≤1MB, auto-compress, no quota).
// If false, uses add_material API (permanent material, no size limit).
func processContentImages(htmlContent string, useUploadImg bool) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}

	var imgs []*html.Node
	var collectImgs func(*html.Node)
	collectImgs = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			imgs = append(imgs, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collectImgs(c)
		}
	}
	collectImgs(doc)

	for _, img := range imgs {
		var src string
		for _, attr := range img.Attr {
			if attr.Key == "src" {
				src = attr.Val
				break
			}
		}
		if src == "" || strings.Contains(src, "mmbiz.qpic.cn") {
			continue
		}

		var label string
		if useUploadImg {
			label = "uploadimg"
		} else {
			label = "add_material"
		}
		fmt.Printf("%s: processing %s\n", label, truncateStr(src, 80))

		imgData, err := downloadImage(src)
		if err != nil {
			fmt.Printf("%s: download failed %s: %v\n", label, truncateStr(src, 80), err)
			continue
		}

		var newURL string
		if useUploadImg {
			// uploadimg: compress if >1MB, then upload
			if len(imgData) > maxImageBytes {
				compressed, err := compressImage(imgData)
				if err != nil {
					fmt.Printf("%s: compress failed %s: %v\n", label, truncateStr(src, 80), err)
					continue
				}
				imgData = compressed
			}
			newURL, err = uploadViaUploadImg(imgData)
		} else {
			// add_material: no size limit, upload directly
			newURL, err = uploadViaAddMaterial(imgData)
		}
		if err != nil {
			fmt.Printf("%s: upload failed %s: %v\n", label, truncateStr(src, 80), err)
			continue
		}

		for i, attr := range img.Attr {
			if attr.Key == "src" {
				img.Attr[i].Val = newURL
				break
			}
		}
		fmt.Printf("%s: replaced -> %s\n", label, truncateStr(newURL, 80))
	}

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return "", fmt.Errorf("render html: %w", err)
	}
	return buf.String(), nil
}

func downloadImage(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func compressImage(data []byte) ([]byte, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	if w > maxImageDim || h > maxImageDim {
		scale := float64(maxImageDim) / float64(max(w, h))
		newW := int(float64(w) * scale)
		newH := int(float64(h) * scale)
		dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
		img = dst
		fmt.Printf("compress: resized %dx%d -> %dx%d\n", w, h, newW, newH)
	}

	for q := defaultJPEGQuality; q >= 10; q -= 10 {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
			return nil, fmt.Errorf("encode: %w", err)
		}
		if buf.Len() <= maxImageBytes {
			fmt.Printf("compress: %s %d -> %d bytes (q=%d)\n", format, len(data), buf.Len(), q)
			return buf.Bytes(), nil
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 10}); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	fmt.Printf("compress: %s %d -> %d bytes (q=10)\n", format, len(data), buf.Len())
	return buf.Bytes(), nil
}

// uploadViaUploadImg uses the media/uploadimg endpoint (no quota, ≤1MB limit).
func uploadViaUploadImg(data []byte) (string, error) {
	var resp uploadImgResp
	err := uploadMultipart(wechatUploadImgURL, data, &resp)
	if err != nil {
		return "", err
	}
	if resp.ErrCode != 0 {
		return "", fmt.Errorf("wechat error %d: %s", resp.ErrCode, resp.ErrMsg)
	}
	return resp.URL, nil
}

// uploadViaAddMaterial uses the material/add_material endpoint (permanent material, no size limit).
func uploadViaAddMaterial(data []byte) (string, error) {
	var resp addMaterialResp
	err := uploadMultipart(wechatAddMaterialURL, data, &resp)
	if err != nil {
		return "", err
	}
	if resp.ErrCode != 0 {
		return "", fmt.Errorf("wechat error %d: %s", resp.ErrCode, resp.ErrMsg)
	}
	return resp.URL, nil
}

// uploadMultipart is a shared helper for multipart/form-data uploads to WeChat.
func uploadMultipart(baseURL string, data []byte, result any) error {
	return WithWeChatToken(func(token string) error {
		url := baseURL + "?access_token=" + token

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("media", "image.jpg")
		if err != nil {
			return err
		}
		if _, err := part.Write(data); err != nil {
			return err
		}
		writer.Close()

		req, err := http.NewRequest("POST", url, &body)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("request: %w", err)
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)

		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		return nil
	})
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
