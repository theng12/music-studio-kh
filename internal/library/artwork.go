package library

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"github.com/dhowden/tag"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
)

const MaxArtworkBytes = 10 << 20

func (l *Library) SetArtwork(id string, source io.Reader) error {
	if _, err := l.Track(id); err != nil {
		return err
	}
	return l.writeArtwork(id, source)
}
func (l *Library) writeArtwork(id string, source io.Reader) error {
	data, err := io.ReadAll(io.LimitReader(source, MaxArtworkBytes+1))
	if err != nil {
		return err
	}
	imageData, err := thumbnail(data)
	if err != nil {
		return err
	}
	dir := filepath.Join(l.dir, "artwork")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".art-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(imageData); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return os.Rename(f.Name(), filepath.Join(dir, id+".png"))
}
func (l *Library) ResetArtwork(id string) error {
	if _, err := l.Track(id); err != nil {
		return err
	}
	err := os.Remove(filepath.Join(l.dir, "artwork", id+".png"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
func (l *Library) Artwork(id string) ([]byte, string, error) {
	t, err := l.Track(id)
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(filepath.Join(l.dir, "artwork", id+".png"))
	if err == nil {
		return data, "image/png", nil
	}
	data, err = os.ReadFile(filepath.Join(l.dir, "artwork", "album-"+albumID(t)+".png"))
	if err == nil {
		return data, "image/png", nil
	}
	f, _, err := l.OpenTrack(id)
	if err != nil {
		return nil, "", err
	}
	m, e := tag.ReadFrom(io.NewSectionReader(f, 0, 16<<20))
	f.Close()
	if e == nil && m.Picture() != nil && len(m.Picture().Data) <= MaxArtworkBytes {
		if thumb, e := l.cachedThumbnail(m.Picture().Data); e == nil {
			return thumb, "image/png", nil
		}
	}
	for _, name := range []string{"cover.jpg", "cover.png", "folder.jpg", "folder.png"} {
		rel := filepath.Join(filepath.Dir(t.Path), name)
		root, rootErr := l.checkedRoot()
		if rootErr != nil {
			return nil, "", rootErr
		}
		f, e := root.Open(rel)
		if e != nil {
			continue
		}
		data, e = io.ReadAll(io.LimitReader(f, MaxArtworkBytes+1))
		f.Close()
		if e == nil && len(data) <= MaxArtworkBytes {
			if thumb, e := l.cachedThumbnail(data); e == nil {
				return thumb, "image/png", nil
			}
		}
	}
	return nil, "", os.ErrNotExist
}

func thumbnail(data []byte) ([]byte, error) {
	if len(data) > MaxArtworkBytes {
		return nil, errors.New("artwork exceeds 10 MiB")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("choose a valid JPEG, PNG, or WebP image")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 16_000_000 {
		return nil, errors.New("artwork dimensions exceed 16 megapixels")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width > 600 || height > 600 {
		if width >= height {
			height = height * 600 / width
			width = 600
		} else {
			width = width * 600 / height
			height = 600
		}
		if width < 1 {
			width = 1
		}
		if height < 1 {
			height = 1
		}
		thumb := image.NewRGBA(image.Rect(0, 0, width, height))
		draw.CatmullRom.Scale(thumb, thumb.Bounds(), img, bounds, draw.Over, nil)
		img = thumb
	}
	var output bytes.Buffer
	if err := png.Encode(&output, img); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
func (l *Library) cachedThumbnail(data []byte) ([]byte, error) {
	key := sha256.Sum256(data)
	l.artMu.Lock()
	defer l.artMu.Unlock()
	if cached, ok := l.artCache[key]; ok {
		return cached, nil
	}
	thumb, e := thumbnail(data)
	if e != nil {
		return nil, e
	}
	if l.artCache == nil {
		l.artCache = map[[32]byte][]byte{}
	}
	if l.artBytes+len(thumb) > 32<<20 || len(l.artCache) >= 128 {
		clear(l.artCache)
		l.artBytes = 0
	}
	l.artCache[key] = thumb
	l.artBytes += len(thumb)
	return thumb, nil
}
