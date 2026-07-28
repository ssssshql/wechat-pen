package server

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/net/html"
)

const (
	cgiUploadImg         = "/cgi-bin/media/uploadimg"
	cgiAddMaterialUpload = "/cgi-bin/material/add_material"
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

		label := "add_material"
		if useUploadImg {
			label = "uploadimg"
		}
		fmt.Printf("%s: processing %s\n", label, truncateStr(src, 80))

		imgData, err := downloadImage(src)
		if err != nil {
			fmt.Printf("%s: download failed %s: %v\n", label, truncateStr(src, 80), err)
			continue
		}

		var newURL string
		if useUploadImg {
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
	return io.ReadAll(io.LimitReader(resp.Body, 10<<20))
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

func uploadViaUploadImg(data []byte) (string, error) {
	var resp uploadImgResp
	if err := wechatPostMultipart(cgiUploadImg, "image.jpg", data, "", &resp); err != nil {
		return "", err
	}
	if resp.ErrCode != 0 {
		return "", fmt.Errorf("wechat error %d: %s", resp.ErrCode, resp.ErrMsg)
	}
	return resp.URL, nil
}

func uploadViaAddMaterial(data []byte) (string, error) {
	var resp addMaterialResp
	if err := wechatPostMultipart(cgiAddMaterialUpload, "image.jpg", data, "type=image", &resp); err != nil {
		return "", err
	}
	if resp.ErrCode != 0 {
		return "", fmt.Errorf("wechat error %d: %s", resp.ErrCode, resp.ErrMsg)
	}
	return resp.URL, nil
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
