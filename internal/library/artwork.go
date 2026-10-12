package library

import (
	"bytes"
	"errors"
	"github.com/dhowden/tag"
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
	data, err := io.ReadAll(io.LimitReader(source, MaxArtworkBytes+1))
	if err != nil {
		return err
	}
	if len(data) > MaxArtworkBytes {
		return errors.New("artwork exceeds 10 MiB")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return errors.New("choose a valid JPEG or PNG image")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 16_000_000 {
		return errors.New("artwork dimensions exceed 16 megapixels")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
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
	if err = png.Encode(f, img); err != nil {
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
	f, _, err := l.OpenTrack(id)
	if err != nil {
		return nil, "", err
	}
	m, e := tag.ReadFrom(f)
	f.Close()
	if e == nil && m.Picture() != nil && len(m.Picture().Data) <= MaxArtworkBytes {
		return m.Picture().Data, m.Picture().MIMEType, nil
	}
	for _, name := range []string{"cover.jpg", "cover.png", "folder.jpg", "folder.png"} {
		rel := filepath.Join(filepath.Dir(t.Path), name)
		f, e := os.OpenInRoot(l.root, rel)
		if e != nil {
			continue
		}
		data, e = io.ReadAll(io.LimitReader(f, MaxArtworkBytes+1))
		f.Close()
		if e == nil && len(data) <= MaxArtworkBytes {
			_, format, e := image.DecodeConfig(bytes.NewReader(data))
			if e == nil {
				return data, "image/" + format, nil
			}
		}
	}
	return nil, "", os.ErrNotExist
}
