package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Call struct {
	ID        int64
	Mode      string
	Task      string
	Result    string
	Tokens    int
	LatencyMs int64
	Error     string
	CreatedAt time.Time
}

type Store struct {
	db *sql.DB
}

func Open() (*Store, error) {
	dir := filepath.Join(os.Getenv("HOME"), ".lm-bridge")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "history.db"))
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS calls (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		mode       TEXT    NOT NULL,
		task       TEXT    NOT NULL,
		result     TEXT    NOT NULL DEFAULT '',
		tokens     INTEGER NOT NULL DEFAULT 0,
		latency_ms INTEGER NOT NULL DEFAULT 0,
		error      TEXT    NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL
	)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS active_task (
		id         INTEGER PRIMARY KEY DEFAULT 1,
		pid        INTEGER NOT NULL,
		mode       TEXT    NOT NULL,
		task       TEXT    NOT NULL,
		progress   REAL    NOT NULL DEFAULT 0,
		started_at INTEGER NOT NULL
	)`)
	return err
}

// ── Active task ───────────────────────────────────────────────────────────────

type ActiveTask struct {
	PID       int
	Mode      string
	Task      string
	Progress  float64
	StartedAt time.Time
}

func (s *Store) SetActiveTask(pid int, mode, task string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO active_task (id, pid, mode, task, progress, started_at)
		 VALUES (1, ?, ?, ?, 0, ?)`,
		pid, mode, task, time.Now().Unix(),
	)
	return err
}

func (s *Store) UpdateTaskProgress(pct float64) error {
	_, err := s.db.Exec(`UPDATE active_task SET progress = ? WHERE id = 1`, pct)
	return err
}

func (s *Store) ClearActiveTask() error {
	_, err := s.db.Exec(`DELETE FROM active_task WHERE id = 1`)
	return err
}

func (s *Store) GetActiveTask() (*ActiveTask, error) {
	var t ActiveTask
	var ts int64
	err := s.db.QueryRow(
		`SELECT pid, mode, task, progress, started_at FROM active_task WHERE id = 1`,
	).Scan(&t.PID, &t.Mode, &t.Task, &t.Progress, &ts)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.StartedAt = time.Unix(ts, 0)
	return &t, nil
}

func (s *Store) GetSetting(key string) (string, error) {
	var val string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}

func (s *Store) SaveCall(c Call) (int64, error) {
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	res, err := s.db.Exec(
		`INSERT INTO calls (mode, task, result, tokens, latency_ms, error, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.Mode, c.Task, c.Result, c.Tokens, c.LatencyMs, c.Error, c.CreatedAt.Unix(),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) RecentCalls(n int) ([]Call, error) {
	rows, err := s.db.Query(
		`SELECT id, mode, task, result, tokens, latency_ms, error, created_at
		 FROM calls ORDER BY id DESC LIMIT ?`, n,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var calls []Call
	for rows.Next() {
		var c Call
		var ts int64
		if err := rows.Scan(&c.ID, &c.Mode, &c.Task, &c.Result, &c.Tokens, &c.LatencyMs, &c.Error, &ts); err != nil {
			return nil, err
		}
		c.CreatedAt = time.Unix(ts, 0)
		calls = append(calls, c)
	}
	return calls, rows.Err()
}

func (s *Store) SessionStats() (totalCalls int, totalTokens int, avgLatencyMs int64, err error) {
	row := s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(tokens),0), COALESCE(AVG(latency_ms),0) FROM calls`,
	)
	err = row.Scan(&totalCalls, &totalTokens, &avgLatencyMs)
	return
}
