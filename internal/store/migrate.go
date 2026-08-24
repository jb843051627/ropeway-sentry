package store

import "fmt"

// schemaDDL 全量建表语句，全部使用 IF NOT EXISTS 幂等执行。
var schemaDDL = []string{
	`CREATE TABLE IF NOT EXISTS ropeway_lines (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		length_m REAL NOT NULL,
		tower_count INTEGER NOT NULL DEFAULT 0,
		rated_speed_ms REAL NOT NULL,
		status TEXT NOT NULL DEFAULT 'open',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS towers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		line_id INTEGER NOT NULL REFERENCES ropeway_lines(id),
		code TEXT NOT NULL,
		height_m REAL NOT NULL,
		position_m REAL NOT NULL,
		tilt_limit_deg REAL NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL,
		UNIQUE(line_id, code)
	)`,
	`CREATE TABLE IF NOT EXISTS rope_sensors (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		line_id INTEGER NOT NULL REFERENCES ropeway_lines(id),
		tower_id INTEGER REFERENCES towers(id),
		code TEXT NOT NULL UNIQUE,
		kind TEXT NOT NULL,
		unit TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 1,
		expected_value REAL NOT NULL DEFAULT 0,
		tolerance REAL NOT NULL DEFAULT 1,
		soft_min REAL NOT NULL,
		soft_max REAL NOT NULL,
		hard_min REAL NOT NULL,
		hard_max REAL NOT NULL,
		created_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS telemetry_batches (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		line_id INTEGER NOT NULL REFERENCES ropeway_lines(id),
		window_start TEXT NOT NULL,
		window_end TEXT NOT NULL,
		point_count INTEGER NOT NULL,
		checksum INTEGER NOT NULL,
		received_at TEXT NOT NULL,
		UNIQUE(line_id, checksum)
	)`,
	`CREATE TABLE IF NOT EXISTS telemetry_points (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		batch_id INTEGER NOT NULL REFERENCES telemetry_batches(id),
		sensor_id INTEGER NOT NULL REFERENCES rope_sensors(id),
		seq INTEGER NOT NULL,
		taken_at TEXT NOT NULL,
		value REAL NOT NULL,
		quality TEXT NOT NULL,
		inserted_at TEXT NOT NULL,
		UNIQUE(batch_id, sensor_id, seq)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_points_sensor_time ON telemetry_points(sensor_id, taken_at)`,
	`CREATE TABLE IF NOT EXISTS vibration_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		line_id INTEGER NOT NULL REFERENCES ropeway_lines(id),
		tower_id INTEGER NOT NULL REFERENCES towers(id),
		peak_accel_ms2 REAL NOT NULL,
		freq_band_hz REAL NOT NULL,
		duration_ms INTEGER NOT NULL,
		severity TEXT NOT NULL,
		occurred_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS cabin_positions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		line_id INTEGER NOT NULL REFERENCES ropeway_lines(id),
		cabin_no TEXT NOT NULL,
		section_m REAL NOT NULL,
		speed_ms REAL NOT NULL,
		gap_to_prev_m REAL NOT NULL,
		recorded_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS tension_baselines (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		line_id INTEGER NOT NULL REFERENCES ropeway_lines(id),
		sensor_code TEXT NOT NULL REFERENCES rope_sensors(code),
		expected_n REAL NOT NULL,
		tolerance_n REAL NOT NULL,
		temp_coeff_n REAL NOT NULL DEFAULT 0,
		ambient_temp_c REAL NOT NULL DEFAULT 20,
		valid_from TEXT NOT NULL,
		valid_to TEXT NOT NULL,
		created_at TEXT NOT NULL,
		UNIQUE(line_id, sensor_code, valid_from)
	)`,
	`CREATE TABLE IF NOT EXISTS safety_assessments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		line_id INTEGER NOT NULL REFERENCES ropeway_lines(id),
		wind_score REAL NOT NULL,
		tension_score REAL NOT NULL,
		structure_score REAL NOT NULL,
		integrity_rate REAL NOT NULL,
		level TEXT NOT NULL,
		icing_active INTEGER NOT NULL DEFAULT 0,
		notes TEXT NOT NULL DEFAULT '',
		assessed_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS alerts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		line_id INTEGER NOT NULL REFERENCES ropeway_lines(id),
		sensor_id INTEGER REFERENCES rope_sensors(id),
		dedup_key TEXT NOT NULL,
		kind TEXT NOT NULL,
		severity TEXT NOT NULL,
		message TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'open',
		occurrences INTEGER NOT NULL DEFAULT 1,
		first_seen_at TEXT NOT NULL,
		latest_seen_at TEXT NOT NULL,
		acked_by TEXT NOT NULL DEFAULT '',
		acked_at TEXT,
		closed_at TEXT,
		close_note TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_alerts_dedup ON alerts(dedup_key, status)`,
	`CREATE TABLE IF NOT EXISTS maintenance_holds (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		line_id INTEGER NOT NULL REFERENCES ropeway_lines(id),
		reason TEXT NOT NULL,
		operator TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'planned',
		planned_at TEXT NOT NULL,
		activated_at TEXT,
		released_at TEXT,
		release_note TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS inspection_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		line_id INTEGER NOT NULL REFERENCES ropeway_lines(id),
		tower_id INTEGER REFERENCES towers(id),
		kind TEXT NOT NULL,
		conclusion TEXT NOT NULL,
		recommendation TEXT NOT NULL DEFAULT '',
		inspector TEXT NOT NULL DEFAULT '',
		inspected_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS sensor_heartbeats (
		sensor_id INTEGER PRIMARY KEY REFERENCES rope_sensors(id),
		value REAL NOT NULL,
		quality TEXT NOT NULL,
		seen_at TEXT NOT NULL
	)`,
}

// migrate 幂等执行全量 DDL。
func (s *Store) migrate() error {
	for i, stmt := range schemaDDL {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("ddl[%d]: %w", i, err)
		}
	}
	return nil
}
