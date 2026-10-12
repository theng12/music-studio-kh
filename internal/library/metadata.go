package library

import (
	"database/sql"
	"encoding/binary"
	"io"
	"math"
	"os"
)

func migrate(db *sql.DB) error {
	rows, e := db.Query("PRAGMA table_info(tracks)")
	if e != nil {
		return e
	}
	columns := map[string]bool{}
	for rows.Next() {
		var index, notnull, primary int
		var name, typ string
		var defaultValue any
		if e = rows.Scan(&index, &name, &typ, &notnull, &defaultValue, &primary); e != nil {
			rows.Close()
			return e
		}
		columns[name] = true
	}
	rows.Close()
	for _, column := range []string{"duration", "modified"} {
		if !columns[column] {
			if _, e = db.Exec("ALTER TABLE tracks ADD COLUMN " + column + " REAL NOT NULL DEFAULT 0"); e != nil {
				return e
			}
		}
	}
	_, e = db.Exec("PRAGMA user_version=2")
	return e
}
func (l *Library) SetDuration(id string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return os.ErrInvalid
	}
	_, e := l.db.Exec("UPDATE tracks SET duration=? WHERE id=?", value, id)
	return e
}
func basicDuration(f *os.File) float64 {
	f.Seek(0, 0)
	defer f.Seek(0, 0)
	header := make([]byte, 44)
	n, _ := io.ReadFull(f, header)
	if n >= 44 && string(header[:4]) == "RIFF" && string(header[8:12]) == "WAVE" && string(header[36:40]) == "data" {
		rate := binary.LittleEndian.Uint32(header[28:32])
		if rate > 0 {
			return float64(binary.LittleEndian.Uint32(header[40:44])) / float64(rate)
		}
	}
	if n >= 26 && string(header[:4]) == "fLaC" {
		bits := binary.BigEndian.Uint64(header[18:26])
		rate := bits >> 44
		if rate > 0 {
			return float64(bits&0xfffffffff) / float64(rate)
		}
	}
	return 0
}
