package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const maxNoteHistory = 12

// NoteSnapshot is a version snapshot of a note.
type NoteSnapshot struct {
	ID       string `json:"id"`
	At       int64  `json:"at"`
	Markdown string `json:"markdown"`
	Title    string `json:"title,omitempty"`
}

// Note is a local note stored in SQLite.
type Note struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Markdown      string         `json:"markdown"`
	UpdatedAt     int64          `json:"updatedAt"`
	Settings      map[string]any `json:"settings,omitempty"`
	History       []NoteSnapshot `json:"history,omitempty"`
	PublishStatus string         `json:"publishStatus"`           // none | draft | published
	MediaID       string         `json:"mediaId,omitempty"`       // wechat draft/publish media_id
	PublishedAt   int64          `json:"publishedAt,omitempty"`   // last status change
	Type          string         `json:"type,omitempty"`          // article | image_post
}

// NormalizePublishStatus maps free-form input to none|draft|published.
func NormalizePublishStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "draft", "wechat_draft", "草稿", "草稿箱":
		return "draft"
	case "published", "publish", "formal", "正式", "已发布":
		return "published"
	default:
		return "none"
	}
}

type notesStore struct {
	db *sql.DB
	mu sync.Mutex
}

var notesDB *notesStore

func notesDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "wechat-pen-notes.db"
	}
	dir := filepath.Join(home, ".wechat-pen")
	_ = os.MkdirAll(dir, 0o700)
	return filepath.Join(dir, "notes.db")
}

func initNotesStore() error {
	path := notesDBPath()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open notes db: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		_ = db.Close()
		return fmt.Errorf("pragma: %w", err)
	}
	schema := `
CREATE TABLE IF NOT EXISTS notes (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL DEFAULT '',
  markdown TEXT NOT NULL DEFAULT '',
  settings TEXT NOT NULL DEFAULT '{}',
  updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS note_history (
  id TEXT PRIMARY KEY,
  note_id TEXT NOT NULL,
  markdown TEXT NOT NULL,
  title TEXT,
  at INTEGER NOT NULL,
  FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_note_history_note ON note_history(note_id, at DESC);
CREATE TABLE IF NOT EXISTS meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return fmt.Errorf("schema: %w", err)
	}
	// migrations for publish tracking (idempotent)
	for _, q := range []string{
		`ALTER TABLE notes ADD COLUMN publish_status TEXT NOT NULL DEFAULT 'none'`,
		`ALTER TABLE notes ADD COLUMN media_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE notes ADD COLUMN published_at INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE notes ADD COLUMN type TEXT NOT NULL DEFAULT 'article'`,
	} {
		_, _ = db.Exec(q) // ignore "duplicate column" on existing DBs
	}
	// writing_styles table for AI writing style profiles
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS writing_styles (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		fakeid TEXT NOT NULL DEFAULT '',
		nickname TEXT NOT NULL DEFAULT '',
		style_prompt TEXT NOT NULL DEFAULT '',
		sample_count INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL DEFAULT 0,
		updated_at INTEGER NOT NULL DEFAULT 0
	)`)
	notesDB = &notesStore{db: db}
	fmt.Printf("notes db → %s\n", path)
	return nil
}

func (s *notesStore) list() ([]Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Load all note rows first and close the cursor before any nested queries.
	// With MaxOpenConns(1), holding rows open while querying history deadlocks.
	rows, err := s.db.Query(`SELECT id, name, markdown, settings, updated_at, publish_status, media_id, published_at, type FROM notes ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	var out []Note
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	for i := range out {
		hist, err := s.loadHistoryLocked(out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].History = hist
	}
	if out == nil {
		out = []Note{}
	}
	return out, nil
}

func (s *notesStore) get(id string) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`SELECT id, name, markdown, settings, updated_at, publish_status, media_id, published_at, type FROM notes WHERE id = ?`, id)
	n, err := scanNote(row)
	if err != nil {
		return Note{}, err
	}
	hist, err := s.loadHistoryLocked(id)
	if err != nil {
		return Note{}, err
	}
	n.History = hist
	return n, nil
}

func (s *notesStore) create(n Note) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n.ID == "" {
		n.ID = fmt.Sprintf("n_%d_%s", time.Now().UnixMilli(), randSuffix())
	}
	if n.UpdatedAt == 0 {
		n.UpdatedAt = time.Now().UnixMilli()
	}
	n.PublishStatus = NormalizePublishStatus(n.PublishStatus)
	settings, _ := json.Marshal(n.Settings)
	if n.Settings == nil {
		settings = []byte("{}")
	}
	_, err := s.db.Exec(
		`INSERT INTO notes (id, name, markdown, settings, updated_at, publish_status, media_id, published_at, type) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Name, n.Markdown, string(settings), n.UpdatedAt, n.PublishStatus, n.MediaID, n.PublishedAt, n.Type,
	)
	if err != nil {
		return Note{}, err
	}
	if n.History == nil {
		n.History = []NoteSnapshot{}
	}
	return n, nil
}

func (s *notesStore) update(id string, name, markdown string, settings map[string]any, pushHist bool, histTitle string, publishStatus *string, mediaID *string) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var oldMD string
	err := s.db.QueryRow(`SELECT markdown FROM notes WHERE id = ?`, id).Scan(&oldMD)
	if err != nil {
		return Note{}, err
	}

	now := time.Now().UnixMilli()
	settingsJSON, _ := json.Marshal(settings)
	if settings == nil {
		settingsJSON = []byte("{}")
	}

	var curStatus, curMedia string
	var curPubAt int64
	_ = s.db.QueryRow(`SELECT publish_status, media_id, published_at FROM notes WHERE id = ?`, id).Scan(&curStatus, &curMedia, &curPubAt)
	if curStatus == "" {
		curStatus = "none"
	}
	newStatus := curStatus
	newMedia := curMedia
	newPubAt := curPubAt
	if publishStatus != nil {
		newStatus = NormalizePublishStatus(*publishStatus)
		if newStatus != curStatus {
			newPubAt = now
		}
	}
	if mediaID != nil {
		newMedia = strings.TrimSpace(*mediaID)
	}

	_, err = s.db.Exec(
		`UPDATE notes SET name = ?, markdown = ?, settings = ?, updated_at = ?, publish_status = ?, media_id = ?, published_at = ? WHERE id = ?`,
		name, markdown, string(settingsJSON), now, newStatus, newMedia, newPubAt, id,
	)
	if err != nil {
		return Note{}, err
	}

	if pushHist && markdown != oldMD {
		snapID := fmt.Sprintf("s_%d_%s", now, randSuffix())
		_, err = s.db.Exec(
			`INSERT INTO note_history (id, note_id, markdown, title, at) VALUES (?, ?, ?, ?, ?)`,
			snapID, id, markdown, histTitle, now,
		)
		if err != nil {
			return Note{}, err
		}
		rows, err := s.db.Query(
			`SELECT id FROM note_history WHERE note_id = ? ORDER BY at DESC`, id,
		)
		if err != nil {
			return Note{}, err
		}
		var ids []string
		for rows.Next() {
			var hid string
			if err := rows.Scan(&hid); err != nil {
				rows.Close()
				return Note{}, err
			}
			ids = append(ids, hid)
		}
		rows.Close()
		if len(ids) > maxNoteHistory {
			for _, hid := range ids[maxNoteHistory:] {
				_, _ = s.db.Exec(`DELETE FROM note_history WHERE id = ?`, hid)
			}
		}
	}

	row := s.db.QueryRow(`SELECT id, name, markdown, settings, updated_at, publish_status, media_id, published_at, type FROM notes WHERE id = ?`, id)
	n, err := scanNote(row)
	if err != nil {
		return Note{}, err
	}
	hist, err := s.loadHistoryLocked(id)
	if err != nil {
		return Note{}, err
	}
	n.History = hist
	return n, nil
}
func (s *notesStore) delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM notes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	active, _ := s.getMetaLocked("active_note_id")
	if active == id {
		_ = s.setMetaLocked("active_note_id", "")
	}
	return nil
}

func (s *notesStore) importNotes(list []Note) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	n := 0
	for _, note := range list {
		if note.ID == "" {
			continue
		}
		settings, _ := json.Marshal(note.Settings)
		if note.Settings == nil {
			settings = []byte("{}")
		}
		if note.UpdatedAt == 0 {
			note.UpdatedAt = time.Now().UnixMilli()
		}
		_, err := tx.Exec(
			`INSERT INTO notes (id, name, markdown, settings, updated_at, publish_status, media_id, published_at, type) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET
			   name=excluded.name,
			   markdown=excluded.markdown,
			   settings=excluded.settings,
			   updated_at=excluded.updated_at,
			   publish_status=excluded.publish_status,
			   media_id=excluded.media_id,
			   published_at=excluded.published_at,
			   type=excluded.type`,
				note.ID, note.Name, note.Markdown, string(settings), note.UpdatedAt, NormalizePublishStatus(note.PublishStatus), note.MediaID, note.PublishedAt, note.Type,
			)
		if err != nil {
			return 0, err
		}
		_, _ = tx.Exec(`DELETE FROM note_history WHERE note_id = ?`, note.ID)
		for i, h := range note.History {
			if i >= maxNoteHistory {
				break
			}
			hid := h.ID
			if hid == "" {
				hid = fmt.Sprintf("s_%d_%s", h.At, randSuffix())
			}
			_, err := tx.Exec(
				`INSERT INTO note_history (id, note_id, markdown, title, at) VALUES (?, ?, ?, ?, ?)`,
				hid, note.ID, h.Markdown, h.Title, h.At,
			)
			if err != nil {
				return 0, err
			}
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *notesStore) getActive() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, _ := s.getMetaLocked("active_note_id")
	return v
}

func (s *notesStore) setActive(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.setMetaLocked("active_note_id", id)
}

func (s *notesStore) getMetaLocked(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func (s *notesStore) setMetaLocked(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		key, value,
	)
	return err
}

func (s *notesStore) loadHistoryLocked(noteID string) ([]NoteSnapshot, error) {
	rows, err := s.db.Query(
		`SELECT id, at, markdown, title FROM note_history WHERE note_id = ? ORDER BY at DESC LIMIT ?`,
		noteID, maxNoteHistory,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NoteSnapshot
	for rows.Next() {
		var h NoteSnapshot
		var title sql.NullString
		if err := rows.Scan(&h.ID, &h.At, &h.Markdown, &title); err != nil {
			return nil, err
		}
		if title.Valid {
			h.Title = title.String
		}
		out = append(out, h)
	}
	if out == nil {
		out = []NoteSnapshot{}
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanNote(row rowScanner) (Note, error) {
	var n Note
	var settings string
	var status, media, noteType string
	var pubAt int64
	if err := row.Scan(&n.ID, &n.Name, &n.Markdown, &settings, &n.UpdatedAt, &status, &media, &pubAt, &noteType); err != nil {
		return Note{}, err
	}
	if settings != "" && settings != "{}" {
		_ = json.Unmarshal([]byte(settings), &n.Settings)
	}
	n.PublishStatus = NormalizePublishStatus(status)
	n.MediaID = media
	n.PublishedAt = pubAt
	n.Type = noteType
	return n, nil
}
func randSuffix() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 5)
	n := time.Now().UnixNano()
	for i := range b {
		b[i] = letters[(n+int64(i)*31)%int64(len(letters))]
		n = n/7 + 97
	}
	return string(b)
}

// --- HTTP handlers ---

func handleNotes(w http.ResponseWriter, r *http.Request) {
	if notesDB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "notes store not ready"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/notes")
	path = strings.Trim(path, "/")

	switch {
	case path == "" && r.Method == http.MethodGet:
		list, err := notesDB.list()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"notes": list, "activeId": notesDB.getActive()})
	case path == "" && r.Method == http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
		defer r.Body.Close()
		var req Note
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		n, err := notesDB.create(req)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = notesDB.setActive(n.ID)
		writeJSON(w, http.StatusOK, n)
	case path == "active" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]string{"id": notesDB.getActive()})
	case path == "active" && r.Method == http.MethodPut:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		defer r.Body.Close()
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := notesDB.setActive(req.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"id": req.ID})
	case path == "import" && r.Method == http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
		defer r.Body.Close()
		var req struct {
			Notes    []Note `json:"notes"`
			ActiveID string `json:"activeId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		n, err := notesDB.importNotes(req.Notes)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if req.ActiveID != "" {
			_ = notesDB.setActive(req.ActiveID)
		}
		writeJSON(w, http.StatusOK, map[string]any{"imported": n, "activeId": notesDB.getActive()})
	case path != "" && !strings.Contains(path, "/") && r.Method == http.MethodGet:
		n, err := notesDB.get(path)
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, n)
	case path != "" && !strings.Contains(path, "/") && r.Method == http.MethodPut:
		r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
		defer r.Body.Close()
		var req struct {
			Name          string         `json:"name"`
			Markdown      string         `json:"markdown"`
			Settings      map[string]any `json:"settings"`
			PushHist      *bool          `json:"pushHistory"`
			HistTitle     string         `json:"historyTitle"`
			PublishStatus *string        `json:"publishStatus"`
			MediaID       *string        `json:"mediaId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		push := true
		if req.PushHist != nil {
			push = *req.PushHist
		}
		n, err := notesDB.update(path, req.Name, req.Markdown, req.Settings, push, req.HistTitle, req.PublishStatus, req.MediaID)
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, n)
	case path != "" && !strings.Contains(path, "/") && r.Method == http.MethodDelete:
		if err := notesDB.delete(path); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
