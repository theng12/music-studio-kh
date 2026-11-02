package library

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestArtistNames(t *testing.T) {
	for _, tc := range []struct {
		credit, albumArtist string
		want                []string
	}{
		{"Ed Sheeran feat. Skrillex", "", []string{"Ed Sheeran", "Skrillex"}},
		{"Ed Sheeran feat. Stromzy", "", []string{"Ed Sheeran", "Stromzy"}},
		{"Ed Sheeran w/ Justin Bieber", "", []string{"Ed Sheeran", "Justin Bieber"}},
		{"Ed Sheeran feat. Paulo Londra & Dave", "", []string{"Ed Sheeran", "Paulo Londra", "Dave"}},
		{"Ed Sheeran ft. Skrillex", "", []string{"Ed Sheeran", "Skrillex"}},
		{"Ed Sheeran featuring Skrillex", "", []string{"Ed Sheeran", "Skrillex"}},
		{"Ed Sheeran with Justin Bieber", "", []string{"Ed Sheeran", "Justin Bieber"}},
		{"Ed Sheeran x Justin Bieber", "", []string{"Ed Sheeran", "Justin Bieber"}},
		{"Simon & Garfunkel", "", []string{"Simon & Garfunkel"}},
		{"Simon & Garfunkel feat. Ed Sheeran", "", []string{"Simon & Garfunkel", "Ed Sheeran"}},
		{"Ed Sheeran & Justin Bieber", "", []string{"Ed Sheeran & Justin Bieber"}},
		{"Guest Singer feat. Ed Sheeran", "Ed Sheeran", []string{"Ed Sheeran", "Guest Singer"}},
		{"Ed Sheeran FEAT. Skrillex & skrillex", " Ed Sheeran ", []string{"Ed Sheeran", "Skrillex"}},
		{"", "", []string{"Unknown artist"}},
	} {
		t.Run(tc.credit+"/"+tc.albumArtist, func(t *testing.T) {
			if got := ArtistNames(tc.credit, tc.albumArtist); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q; want %q", got, tc.want)
			}
		})
	}
}

func TestAlbumArtistScanKeepsFullCreditAndFile(t *testing.T) {
	l, root := openTest(t)
	credit := "Guest Singer feat. Other Guest"
	var frames bytes.Buffer
	for _, frame := range []struct{ id, value string }{{"TIT2", "Collaboration"}, {"TPE1", credit}, {"TPE2", "Ed Sheeran"}, {"TALB", "Collaborations"}} {
		frames.WriteString(frame.id)
		binary.Write(&frames, binary.BigEndian, uint32(len(frame.value)+1))
		frames.Write([]byte{0, 0, 0})
		frames.WriteString(frame.value)
	}
	size := frames.Len()
	original := append([]byte{'I', 'D', '3', 3, 0, 0, byte(size>>21) & 127, byte(size>>14) & 127, byte(size>>7) & 127, byte(size) & 127}, frames.Bytes()...)
	file := filepath.Join(root, "Collaboration.mp3")
	if err := os.WriteFile(file, original, 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := l.Scan(); err != nil {
			t.Fatal(err)
		}
		tracks, err := l.Tracks("Ed Sheeran")
		if err != nil || len(tracks) != 1 {
			t.Fatalf("tracks: %+v %v", tracks, err)
		}
		track, err := l.Track(tracks[0].ID)
		if err != nil || track.Artist != credit || track.AlbumArtist != "Ed Sheeran" || !reflect.DeepEqual(track.Artists, []string{"Ed Sheeran", "Guest Singer", "Other Guest"}) {
			t.Fatalf("full credit/tag/grouping lost: %+v %v", track, err)
		}
		if _, err := l.UpdateTrackMetadata(track.ID, MetadataUpdate{Title: track.Title, Artist: credit, Album: track.Album}); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			// Simulate an existing 0.2.2 index, then backfill unchanged files.
			if _, err := l.db.Exec("ALTER TABLE tracks DROP COLUMN album_artist"); err != nil {
				t.Fatal(err)
			}
			if err := migrate(l.db); err != nil {
				t.Fatal(err)
			}
		}
	}
	data, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatalf("audio tags changed: %v", err)
	}
}
