package app

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

type Server struct {
	db     *sql.DB
	sql    *sqlLogger
	dbPath string
}

func NewServer(dbPath string) (*Server, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	server := &Server{db: db, sql: newSQLLogger(db, os.Getenv("CLASSPOINTS_SQL_LOG") != "0"), dbPath: dbPath}
	if err := server.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := server.ensureSettings(filepath.Join(filepath.Dir(dbPath), "backups")); err != nil {
		_ = db.Close()
		return nil, err
	}
	return server, nil
}

func (s *Server) Close() error {
	return s.db.Close()
}

func (s *Server) RegisterRoutes(api *gin.RouterGroup) {
	api.POST("/login", s.login)
	api.GET("/classes", s.listClasses)
	api.POST("/classes", s.createClass)
	api.DELETE("/classes/:classID", s.deleteClass)
	api.GET("/classes/:classID/dashboard", s.dashboard)

	api.GET("/classes/:classID/students", s.listStudents)
	api.POST("/classes/:classID/students", s.createStudent)
	api.PUT("/students/:studentID", s.updateStudent)
	api.DELETE("/students/:studentID", s.deleteStudent)
	api.GET("/classes/:classID/groups", s.listStudentGroups)
	api.POST("/classes/:classID/groups", s.createStudentGroup)
	api.PUT("/groups/:groupID", s.updateStudentGroup)

	api.GET("/classes/:classID/rules", s.listRules)
	api.POST("/classes/:classID/rules", s.createRule)
	api.PUT("/rules/:ruleID", s.updateRule)
	api.DELETE("/rules/:ruleID", s.deleteRule)

	api.GET("/classes/:classID/records", s.listRecords)
	api.POST("/classes/:classID/records", s.createRecord)
	api.POST("/classes/:classID/records/batch", s.createBatchRecords)
	api.POST("/records/:recordID/undo", s.undoRecord)
	api.GET("/classes/:classID/ranking", s.ranking)

	api.GET("/settings", s.getSettings)
	api.PUT("/settings", s.updateSettings)
	api.POST("/backup", s.manualBackup)
}

func (s *Server) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS classes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	teacher TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS students (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	class_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	code TEXT NOT NULL DEFAULT '',
	group_name TEXT NOT NULL DEFAULT '',
	gender TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	FOREIGN KEY(class_id) REFERENCES classes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS student_groups (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	class_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	created_at TEXT NOT NULL,
	UNIQUE(class_id, name),
	FOREIGN KEY(class_id) REFERENCES classes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS rules (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	class_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	score INTEGER NOT NULL,
	category TEXT NOT NULL DEFAULT '',
	enabled INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	FOREIGN KEY(class_id) REFERENCES classes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS score_records (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	class_id INTEGER NOT NULL,
	student_id INTEGER NOT NULL,
	rule_id INTEGER,
	rule_name TEXT NOT NULL DEFAULT '',
	score INTEGER NOT NULL,
	reason TEXT NOT NULL,
	note TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	undone_at TEXT,
	undo_reason TEXT NOT NULL DEFAULT '',
	FOREIGN KEY(class_id) REFERENCES classes(id) ON DELETE CASCADE,
	FOREIGN KEY(student_id) REFERENCES students(id) ON DELETE CASCADE,
	FOREIGN KEY(rule_id) REFERENCES rules(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`
	_, err := s.sql.Exec(schema)
	if err != nil {
		return err
	}
	if err := s.ensureColumn("students", "gender", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("classes", "deleted_at", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("students", "group_name", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if _, err := s.sql.Exec(`UPDATE students SET group_name = '' WHERE TRIM(group_name) = '未分组'`); err != nil {
		return err
	}
	_, err = s.sql.Exec(`
INSERT OR IGNORE INTO student_groups(class_id, name, created_at)
SELECT class_id, TRIM(group_name), MIN(created_at)
FROM students
WHERE TRIM(group_name) <> ''
GROUP BY class_id, TRIM(group_name)`)
	return err
}

func (s *Server) ensureColumn(table string, column string, definition string) error {
	rows, err := s.sql.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			found = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = s.sql.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + definition)
	return err
}

func (s *Server) ensureSettings(defaultBackupDir string) error {
	defaults := map[string]string{
		"schoolName":      "我的学校",
		"backupDir":       defaultBackupDir,
		"autoBackupMins":  "30",
		"backupKeepCount": "20",
	}
	for key, value := range defaults {
		if _, err := s.sql.Exec(`INSERT OR IGNORE INTO settings(key, value) VALUES(?, ?)`, key, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) login(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	_ = c.ShouldBindJSON(&req)
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "老师"
	}
	c.JSON(http.StatusOK, gin.H{"name": req.Name})
}

func (s *Server) listClasses(c *gin.Context) {
	rows, err := s.sql.Query(`SELECT id, name, teacher, created_at FROM classes WHERE deleted_at IS NULL ORDER BY created_at DESC`)
	if err != nil {
		respondError(c, err)
		return
	}
	defer rows.Close()

	classes := make([]Class, 0)
	for rows.Next() {
		item, err := scanClass(rows)
		if err != nil {
			respondError(c, err)
			return
		}
		classes = append(classes, item)
	}
	c.JSON(http.StatusOK, classes)
}

func (s *Server) createClass(c *gin.Context) {
	var req struct {
		Name    string `json:"name"`
		Teacher string `json:"teacher"`
	}
	if !bindJSON(c, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		respondBadRequest(c, "班级名称不能为空")
		return
	}

	now := time.Now()
	result, err := s.sql.Exec(`INSERT INTO classes(name, teacher, created_at) VALUES(?, ?, ?)`, req.Name, strings.TrimSpace(req.Teacher), formatTime(now))
	if err != nil {
		respondError(c, err)
		return
	}
	id, _ := result.LastInsertId()
	s.afterMutation()
	c.JSON(http.StatusCreated, Class{ID: id, Name: req.Name, Teacher: strings.TrimSpace(req.Teacher), CreatedAt: now})
}

func (s *Server) deleteClass(c *gin.Context) {
	classID, ok := parseIDParam(c, "classID")
	if !ok {
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		respondError(c, err)
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM score_records WHERE class_id = ?`, classID); err != nil {
		respondError(c, err)
		return
	}
	if _, err := tx.Exec(`DELETE FROM students WHERE class_id = ?`, classID); err != nil {
		respondError(c, err)
		return
	}
	result, err := tx.Exec(`UPDATE classes SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`, formatTime(time.Now()), classID)
	if err != nil {
		respondError(c, err)
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		respondError(c, err)
		return
	}
	if affected == 0 {
		respondBadRequest(c, "班级不存在或已删除")
		return
	}
	if err := tx.Commit(); err != nil {
		respondError(c, err)
		return
	}

	s.afterMutation()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) dashboard(c *gin.Context) {
	classID, ok := parseIDParam(c, "classID")
	if !ok {
		return
	}
	class, err := s.getClass(classID)
	if err != nil {
		respondError(c, err)
		return
	}
	students, err := s.queryRanking(classID, "", "")
	if err != nil {
		respondError(c, err)
		return
	}
	recent, err := s.queryRecords(classID, "all", 0, "", "", 8)
	if err != nil {
		respondError(c, err)
		return
	}
	total := 0
	for _, student := range students {
		total += student.Score
	}
	var leader *Student
	if len(students) > 0 {
		leader = &students[0]
	}
	todayCount, err := s.countTodayRecords(classID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, Dashboard{
		Class:         class,
		StudentCount:  len(students),
		TotalScore:    total,
		TodayRecords:  todayCount,
		Leader:        leader,
		RecentRecords: recent,
		Ranking:       students,
	})
}

func (s *Server) getClass(classID int64) (Class, error) {
	row := s.sql.QueryRow(`SELECT id, name, teacher, created_at FROM classes WHERE id = ? AND deleted_at IS NULL`, classID)
	return scanClass(row)
}

func (s *Server) listStudents(c *gin.Context) {
	classID, ok := parseIDParam(c, "classID")
	if !ok {
		return
	}
	students, err := s.queryStudents(classID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, students)
}

func (s *Server) createStudent(c *gin.Context) {
	classID, ok := parseIDParam(c, "classID")
	if !ok {
		return
	}
	var req struct {
		Name      string `json:"name"`
		Code      string `json:"code"`
		GroupName string `json:"groupName"`
		Gender    string `json:"gender"`
	}
	if !bindJSON(c, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.GroupName = normalizeGroupName(req.GroupName)
	if req.Name == "" {
		respondBadRequest(c, "学生姓名不能为空")
		return
	}
	if err := s.ensureStudentGroup(classID, req.GroupName); err != nil {
		respondError(c, err)
		return
	}

	now := time.Now()
	result, err := s.sql.Exec(
		`INSERT INTO students(class_id, name, code, group_name, gender, created_at) VALUES(?, ?, ?, ?, ?, ?)`,
		classID, req.Name, strings.TrimSpace(req.Code), req.GroupName, normalizeGender(req.Gender), formatTime(now),
	)
	if err != nil {
		respondError(c, err)
		return
	}
	id, _ := result.LastInsertId()
	s.afterMutation()
	c.JSON(http.StatusCreated, Student{ID: id, ClassID: classID, Name: req.Name, Code: strings.TrimSpace(req.Code), GroupName: req.GroupName, Gender: normalizeGender(req.Gender), CreatedAt: now})
}

func (s *Server) updateStudent(c *gin.Context) {
	studentID, ok := parseIDParam(c, "studentID")
	if !ok {
		return
	}
	var req struct {
		Name      string `json:"name"`
		Code      string `json:"code"`
		GroupName string `json:"groupName"`
		Gender    string `json:"gender"`
	}
	if !bindJSON(c, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.GroupName = normalizeGroupName(req.GroupName)
	if req.Name == "" {
		respondBadRequest(c, "学生姓名不能为空")
		return
	}
	var classID int64
	if err := s.sql.QueryRow(`SELECT class_id FROM students WHERE id = ?`, studentID).Scan(&classID); err != nil {
		respondError(c, err)
		return
	}
	if err := s.ensureStudentGroup(classID, req.GroupName); err != nil {
		respondError(c, err)
		return
	}
	_, err := s.sql.Exec(`UPDATE students SET name = ?, code = ?, group_name = ?, gender = ? WHERE id = ?`, req.Name, strings.TrimSpace(req.Code), req.GroupName, normalizeGender(req.Gender), studentID)
	if err != nil {
		respondError(c, err)
		return
	}
	s.afterMutation()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) deleteStudent(c *gin.Context) {
	studentID, ok := parseIDParam(c, "studentID")
	if !ok {
		return
	}
	_, err := s.sql.Exec(`DELETE FROM students WHERE id = ?`, studentID)
	if err != nil {
		respondError(c, err)
		return
	}
	s.afterMutation()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) listStudentGroups(c *gin.Context) {
	classID, ok := parseIDParam(c, "classID")
	if !ok {
		return
	}
	groups, err := s.queryStudentGroups(classID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, groups)
}

func (s *Server) createStudentGroup(c *gin.Context) {
	classID, ok := parseIDParam(c, "classID")
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !bindJSON(c, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		respondBadRequest(c, "小组名称不能为空")
		return
	}
	if req.Name == "未分组" {
		respondBadRequest(c, "“未分组”是系统保留名称")
		return
	}
	var count int
	if err := s.sql.QueryRow(`SELECT COUNT(*) FROM student_groups WHERE class_id = ? AND name = ?`, classID, req.Name).Scan(&count); err != nil {
		respondError(c, err)
		return
	}
	if count > 0 {
		respondBadRequest(c, "小组名称已存在")
		return
	}
	now := time.Now()
	result, err := s.sql.Exec(`INSERT INTO student_groups(class_id, name, created_at) VALUES(?, ?, ?)`, classID, req.Name, formatTime(now))
	if err != nil {
		respondError(c, err)
		return
	}
	id, _ := result.LastInsertId()
	s.afterMutation()
	c.JSON(http.StatusCreated, StudentGroup{ID: id, ClassID: classID, Name: req.Name, CreatedAt: now})
}

func (s *Server) updateStudentGroup(c *gin.Context) {
	groupID, ok := parseIDParam(c, "groupID")
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !bindJSON(c, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		respondBadRequest(c, "小组名称不能为空")
		return
	}
	if req.Name == "未分组" {
		respondBadRequest(c, "“未分组”是系统保留名称")
		return
	}

	var classID int64
	var oldName string
	if err := s.sql.QueryRow(`SELECT class_id, name FROM student_groups WHERE id = ?`, groupID).Scan(&classID, &oldName); err != nil {
		respondError(c, err)
		return
	}
	var count int
	if err := s.sql.QueryRow(`SELECT COUNT(*) FROM student_groups WHERE class_id = ? AND name = ? AND id <> ?`, classID, req.Name, groupID).Scan(&count); err != nil {
		respondError(c, err)
		return
	}
	if count > 0 {
		respondBadRequest(c, "小组名称已存在")
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		respondError(c, err)
		return
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE student_groups SET name = ? WHERE id = ?`, req.Name, groupID); err != nil {
		respondError(c, err)
		return
	}
	if _, err := tx.Exec(`UPDATE students SET group_name = ? WHERE class_id = ? AND group_name = ?`, req.Name, classID, oldName); err != nil {
		respondError(c, err)
		return
	}
	if err := tx.Commit(); err != nil {
		respondError(c, err)
		return
	}
	s.afterMutation()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) listRules(c *gin.Context) {
	classID, ok := parseIDParam(c, "classID")
	if !ok {
		return
	}
	rules, err := s.queryRules(classID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (s *Server) createRule(c *gin.Context) {
	classID, ok := parseIDParam(c, "classID")
	if !ok {
		return
	}
	var req struct {
		Name     string `json:"name"`
		Score    int    `json:"score"`
		Category string `json:"category"`
	}
	if !bindJSON(c, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.Score == 0 {
		respondBadRequest(c, "规则名称不能为空，分值不能为 0")
		return
	}
	now := time.Now()
	result, err := s.sql.Exec(
		`INSERT INTO rules(class_id, name, score, category, enabled, created_at) VALUES(?, ?, ?, ?, 1, ?)`,
		classID, req.Name, req.Score, strings.TrimSpace(req.Category), formatTime(now),
	)
	if err != nil {
		respondError(c, err)
		return
	}
	id, _ := result.LastInsertId()
	s.afterMutation()
	c.JSON(http.StatusCreated, Rule{ID: id, ClassID: classID, Name: req.Name, Score: req.Score, Category: req.Category, Enabled: true, CreatedAt: now})
}

func (s *Server) updateRule(c *gin.Context) {
	ruleID, ok := parseIDParam(c, "ruleID")
	if !ok {
		return
	}
	var req struct {
		Name     string `json:"name"`
		Score    int    `json:"score"`
		Category string `json:"category"`
		Enabled  bool   `json:"enabled"`
	}
	if !bindJSON(c, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.Score == 0 {
		respondBadRequest(c, "规则名称不能为空，分值不能为 0")
		return
	}
	_, err := s.sql.Exec(`UPDATE rules SET name = ?, score = ?, category = ?, enabled = ? WHERE id = ?`, req.Name, req.Score, strings.TrimSpace(req.Category), boolToInt(req.Enabled), ruleID)
	if err != nil {
		respondError(c, err)
		return
	}
	s.afterMutation()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) deleteRule(c *gin.Context) {
	ruleID, ok := parseIDParam(c, "ruleID")
	if !ok {
		return
	}
	_, err := s.sql.Exec(`DELETE FROM rules WHERE id = ?`, ruleID)
	if err != nil {
		respondError(c, err)
		return
	}
	s.afterMutation()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) listRecords(c *gin.Context) {
	classID, ok := parseIDParam(c, "classID")
	if !ok {
		return
	}
	studentID, err := parseOptionalPositiveInt64(c.Query("studentId"))
	if err != nil {
		respondBadRequest(c, "学生筛选参数无效")
		return
	}
	startDate := strings.TrimSpace(c.Query("startDate"))
	endDate := strings.TrimSpace(c.Query("endDate"))
	if !validOptionalDate(startDate) || !validOptionalDate(endDate) {
		respondBadRequest(c, "日期格式无效")
		return
	}
	if startDate != "" && endDate != "" && startDate > endDate {
		respondBadRequest(c, "开始日期不能晚于结束日期")
		return
	}
	records, err := s.queryRecords(classID, c.DefaultQuery("filter", "all"), studentID, startDate, endDate, 0)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, records)
}

func (s *Server) createRecord(c *gin.Context) {
	classID, ok := parseIDParam(c, "classID")
	if !ok {
		return
	}
	var req recordRequest
	if !bindJSON(c, &req) {
		return
	}
	record, err := s.insertRecord(classID, req)
	if err != nil {
		respondRequestError(c, err)
		return
	}
	s.afterMutation()
	c.JSON(http.StatusCreated, record)
}

func (s *Server) createBatchRecords(c *gin.Context) {
	classID, ok := parseIDParam(c, "classID")
	if !ok {
		return
	}
	var req struct {
		StudentIDs []int64 `json:"studentIds"`
		RuleID     *int64  `json:"ruleId"`
		Score      int     `json:"score"`
		Reason     string  `json:"reason"`
		Note       string  `json:"note"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if len(req.StudentIDs) == 0 {
		respondBadRequest(c, "请选择学生")
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		respondError(c, err)
		return
	}
	defer tx.Rollback()

	records := make([]ScoreRecord, 0)
	for _, studentID := range req.StudentIDs {
		record, err := s.insertRecordWithDB(newTxLogger(tx, s.sql.enabled), classID, recordRequest{
			StudentID: studentID,
			RuleID:    req.RuleID,
			Score:     req.Score,
			Reason:    req.Reason,
			Note:      req.Note,
		})
		if err != nil {
			respondRequestError(c, err)
			return
		}
		records = append(records, record)
	}
	if err := tx.Commit(); err != nil {
		respondError(c, err)
		return
	}
	s.afterMutation()
	c.JSON(http.StatusCreated, records)
}

func (s *Server) undoRecord(c *gin.Context) {
	recordID, ok := parseIDParam(c, "recordID")
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	now := time.Now()
	result, err := s.sql.Exec(`UPDATE score_records SET undone_at = ?, undo_reason = ? WHERE id = ? AND undone_at IS NULL`, formatTime(now), strings.TrimSpace(req.Reason), recordID)
	if err != nil {
		respondError(c, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		respondBadRequest(c, "记录不存在或已撤销")
		return
	}
	s.afterMutation()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) ranking(c *gin.Context) {
	classID, ok := parseIDParam(c, "classID")
	if !ok {
		return
	}
	scope := c.DefaultQuery("scope", "all")
	startDate := strings.TrimSpace(c.Query("startDate"))
	endDate := strings.TrimSpace(c.Query("endDate"))
	if scope == "custom" {
		if !validOptionalDate(startDate) || !validOptionalDate(endDate) {
			respondBadRequest(c, "日期格式无效")
			return
		}
		if startDate != "" && endDate != "" && startDate > endDate {
			respondBadRequest(c, "开始日期不能晚于结束日期")
			return
		}
	} else {
		startDate, endDate = rankingDateRange(scope, time.Now())
	}
	ranking, err := s.queryRanking(classID, startDate, endDate)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, ranking)
}

func (s *Server) getSettings(c *gin.Context) {
	settings, err := s.readSettings()
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (s *Server) updateSettings(c *gin.Context) {
	var req Settings
	if !bindJSON(c, &req) {
		return
	}
	if req.AutoBackupMins <= 0 {
		req.AutoBackupMins = 30
	}
	if req.BackupKeepCount <= 0 {
		req.BackupKeepCount = 20
	}
	values := map[string]string{
		"schoolName":      strings.TrimSpace(req.SchoolName),
		"backupDir":       strings.TrimSpace(req.BackupDir),
		"autoBackupMins":  strconv.Itoa(req.AutoBackupMins),
		"backupKeepCount": strconv.Itoa(req.BackupKeepCount),
	}
	for key, value := range values {
		if _, err := s.sql.Exec(`INSERT INTO settings(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value); err != nil {
			respondError(c, err)
			return
		}
	}
	s.afterMutation()
	c.JSON(http.StatusOK, req)
}

func (s *Server) manualBackup(c *gin.Context) {
	settings, err := s.readSettings()
	if err != nil {
		respondError(c, err)
		return
	}
	path, err := s.BackupNow(settings.BackupDir, settings.BackupKeepCount)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": path})
}

type recordRequest struct {
	StudentID int64  `json:"studentId"`
	RuleID    *int64 `json:"ruleId"`
	Score     int    `json:"score"`
	Reason    string `json:"reason"`
	Note      string `json:"note"`
}

type dbRunner interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

func (s *Server) insertRecord(classID int64, req recordRequest) (ScoreRecord, error) {
	return s.insertRecordWithDB(s.db, classID, req)
}

func (s *Server) insertRecordWithDB(db dbRunner, classID int64, req recordRequest) (ScoreRecord, error) {
	req.Reason = strings.TrimSpace(req.Reason)
	if req.StudentID == 0 || req.Reason == "" {
		return ScoreRecord{}, errBadRequest("学生和原因不能为空")
	}

	var student Student
	row := db.QueryRow(`SELECT id, class_id, name, code, gender, created_at FROM students WHERE id = ? AND class_id = ?`, req.StudentID, classID)
	if err := scanStudentInto(row, &student); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ScoreRecord{}, errBadRequest("学生不存在")
		}
		return ScoreRecord{}, err
	}

	ruleName := "自定义"
	if req.RuleID != nil {
		var ruleScore int
		err := db.QueryRow(`SELECT name, score FROM rules WHERE id = ? AND class_id = ?`, *req.RuleID, classID).Scan(&ruleName, &ruleScore)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ScoreRecord{}, errBadRequest("规则不存在")
			}
			return ScoreRecord{}, err
		}
		if req.Score == 0 {
			req.Score = ruleScore
		}
	}
	if req.Score == 0 {
		return ScoreRecord{}, errBadRequest("分值不能为 0")
	}

	now := time.Now()
	result, err := db.Exec(
		`INSERT INTO score_records(class_id, student_id, rule_id, rule_name, score, reason, note, created_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		classID, req.StudentID, req.RuleID, ruleName, req.Score, req.Reason, strings.TrimSpace(req.Note), formatTime(now),
	)
	if err != nil {
		return ScoreRecord{}, err
	}
	id, _ := result.LastInsertId()
	return ScoreRecord{
		ID:          id,
		ClassID:     classID,
		StudentID:   req.StudentID,
		StudentName: student.Name,
		Gender:      student.Gender,
		RuleID:      req.RuleID,
		RuleName:    ruleName,
		Score:       req.Score,
		Reason:      req.Reason,
		Note:        strings.TrimSpace(req.Note),
		CreatedAt:   now,
		Effective:   true,
	}, nil
}

func (s *Server) queryStudents(classID int64) ([]Student, error) {
	rows, err := s.sql.Query(`
SELECT s.id, s.class_id, s.name, s.code, s.group_name, s.gender, s.created_at,
       COALESCE(SUM(CASE WHEN r.undone_at IS NULL THEN r.score ELSE 0 END), 0) AS score
FROM students s
LEFT JOIN score_records r ON r.student_id = s.id
WHERE s.class_id = ?
GROUP BY s.id
ORDER BY CASE WHEN TRIM(s.group_name) = '' THEN 1 ELSE 0 END, s.group_name, s.code, s.name`, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	students := make([]Student, 0)
	for rows.Next() {
		var student Student
		var created string
		if err := rows.Scan(&student.ID, &student.ClassID, &student.Name, &student.Code, &student.GroupName, &student.Gender, &created, &student.Score); err != nil {
			return nil, err
		}
		student.CreatedAt = parseTime(created)
		students = append(students, student)
	}
	return students, rows.Err()
}

func (s *Server) queryStudentGroups(classID int64) ([]StudentGroup, error) {
	rows, err := s.sql.Query(`
SELECT g.id, g.class_id, g.name, g.created_at, COUNT(s.id)
FROM student_groups g
LEFT JOIN students s ON s.class_id = g.class_id AND s.group_name = g.name
WHERE g.class_id = ?
GROUP BY g.id
ORDER BY g.id`, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := make([]StudentGroup, 0)
	for rows.Next() {
		var group StudentGroup
		var created string
		if err := rows.Scan(&group.ID, &group.ClassID, &group.Name, &created, &group.StudentCount); err != nil {
			return nil, err
		}
		group.CreatedAt = parseTime(created)
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (s *Server) ensureStudentGroup(classID int64, groupName string) error {
	groupName = normalizeGroupName(groupName)
	if groupName == "" {
		return nil
	}
	_, err := s.sql.Exec(
		`INSERT OR IGNORE INTO student_groups(class_id, name, created_at) VALUES(?, ?, ?)`,
		classID, groupName, formatTime(time.Now()),
	)
	return err
}

func (s *Server) queryRules(classID int64) ([]Rule, error) {
	rows, err := s.sql.Query(`SELECT id, class_id, name, score, category, enabled, created_at FROM rules WHERE class_id = ? ORDER BY enabled DESC, category, name`, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]Rule, 0)
	for rows.Next() {
		var rule Rule
		var enabled int
		var created string
		if err := rows.Scan(&rule.ID, &rule.ClassID, &rule.Name, &rule.Score, &rule.Category, &enabled, &created); err != nil {
			return nil, err
		}
		rule.Enabled = enabled == 1
		rule.CreatedAt = parseTime(created)
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (s *Server) queryRecords(classID int64, filter string, studentID int64, startDate, endDate string, limit int) ([]ScoreRecord, error) {
	where := []string{"r.class_id = ?"}
	args := []any{classID}
	switch filter {
	case "today":
		where = append(where, "date(r.created_at) = date('now', 'localtime')")
	case "positive":
		where = append(where, "r.score > 0")
	case "negative":
		where = append(where, "r.score < 0")
	case "undone":
		where = append(where, "r.undone_at IS NOT NULL")
	}
	if studentID > 0 {
		where = append(where, "r.student_id = ?")
		args = append(args, studentID)
	}
	if startDate != "" {
		where = append(where, "substr(r.created_at, 1, 10) >= ?")
		args = append(args, startDate)
	}
	if endDate != "" {
		where = append(where, "substr(r.created_at, 1, 10) <= ?")
		args = append(args, endDate)
	}
	query := fmt.Sprintf(`
SELECT r.id, r.class_id, r.student_id, COALESCE(s.name, '已删除学生'), COALESCE(s.gender, ''), r.rule_id, r.rule_name, r.score, r.reason, r.note, r.created_at, r.undone_at, r.undo_reason
FROM score_records r
LEFT JOIN students s ON s.id = r.student_id
WHERE %s
ORDER BY r.created_at DESC, r.id DESC`, strings.Join(where, " AND "))
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := s.sql.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]ScoreRecord, 0)
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Server) queryRanking(classID int64, startDate, endDate string) ([]Student, error) {
	joinConditions := []string{"r.student_id = s.id"}
	args := make([]any, 0, 3)
	if startDate != "" {
		joinConditions = append(joinConditions, "substr(r.created_at, 1, 10) >= ?")
		args = append(args, startDate)
	}
	if endDate != "" {
		joinConditions = append(joinConditions, "substr(r.created_at, 1, 10) <= ?")
		args = append(args, endDate)
	}
	args = append(args, classID)
	rows, err := s.sql.Query(fmt.Sprintf(`
SELECT s.id, s.class_id, s.name, s.code, s.gender, s.created_at,
       COALESCE(SUM(CASE WHEN r.undone_at IS NULL THEN r.score ELSE 0 END), 0) AS score
FROM students s
LEFT JOIN score_records r ON %s
WHERE s.class_id = ?
GROUP BY s.id
ORDER BY score DESC, s.gender, s.code, s.name`, strings.Join(joinConditions, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	students := make([]Student, 0)
	for rows.Next() {
		var student Student
		var created string
		if err := rows.Scan(&student.ID, &student.ClassID, &student.Name, &student.Code, &student.Gender, &created, &student.Score); err != nil {
			return nil, err
		}
		student.CreatedAt = parseTime(created)
		students = append(students, student)
	}
	return students, rows.Err()
}

func (s *Server) countTodayRecords(classID int64) (int, error) {
	var count int
	err := s.sql.QueryRow(`SELECT COUNT(*) FROM score_records WHERE class_id = ? AND date(created_at) = date('now', 'localtime')`, classID).Scan(&count)
	return count, err
}

func (s *Server) readSettings() (Settings, error) {
	rows, err := s.sql.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return Settings{}, err
	}
	defer rows.Close()

	values := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return Settings{}, err
		}
		values[key] = value
	}
	autoBackupMins, _ := strconv.Atoi(values["autoBackupMins"])
	keepCount, _ := strconv.Atoi(values["backupKeepCount"])
	if autoBackupMins <= 0 {
		autoBackupMins = 30
	}
	if keepCount <= 0 {
		keepCount = 20
	}
	return Settings{
		SchoolName:      values["schoolName"],
		BackupDir:       values["backupDir"],
		AutoBackupMins:  autoBackupMins,
		BackupKeepCount: keepCount,
	}, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanClass(row scanner) (Class, error) {
	var item Class
	var created string
	if err := row.Scan(&item.ID, &item.Name, &item.Teacher, &created); err != nil {
		return item, err
	}
	item.CreatedAt = parseTime(created)
	return item, nil
}

func scanStudentInto(row scanner, student *Student) error {
	var created string
	if err := row.Scan(&student.ID, &student.ClassID, &student.Name, &student.Code, &student.Gender, &created); err != nil {
		return err
	}
	student.CreatedAt = parseTime(created)
	return nil
}

func scanRecord(row scanner) (ScoreRecord, error) {
	var record ScoreRecord
	var ruleID sql.NullInt64
	var created string
	var undone sql.NullString
	if err := row.Scan(&record.ID, &record.ClassID, &record.StudentID, &record.StudentName, &record.Gender, &ruleID, &record.RuleName, &record.Score, &record.Reason, &record.Note, &created, &undone, &record.UndoReason); err != nil {
		return record, err
	}
	if ruleID.Valid {
		record.RuleID = &ruleID.Int64
	}
	record.CreatedAt = parseTime(created)
	if undone.Valid && undone.String != "" {
		parsed := parseTime(undone.String)
		record.UndoneAt = &parsed
		record.Effective = false
		record.EffectiveTxt = "已撤销"
	} else {
		record.Effective = true
		record.EffectiveTxt = "有效"
	}
	return record, nil
}

func parseIDParam(c *gin.Context, key string) (int64, bool) {
	value, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil || value <= 0 {
		respondBadRequest(c, "参数不正确")
		return 0, false
	}
	return value, true
}

func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		respondBadRequest(c, "请求数据不正确")
		return false
	}
	return true
}

type requestError string

func (e requestError) Error() string {
	return string(e)
}

func errBadRequest(message string) error {
	return requestError(message)
}

func respondRequestError(c *gin.Context, err error) {
	var reqErr requestError
	if errors.As(err, &reqErr) {
		respondBadRequest(c, reqErr.Error())
		return
	}
	respondError(c, err)
}

func respondBadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{"error": message})
}

func respondError(c *gin.Context, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "数据不存在"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

func formatTime(value time.Time) string {
	return value.Format(time.RFC3339)
}

func parseOptionalPositiveInt64(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid positive integer")
	}
	return parsed, nil
}

func validOptionalDate(value string) bool {
	if value == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

func rankingDateRange(scope string, now time.Time) (string, string) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endDate := today.Format("2006-01-02")
	switch scope {
	case "today":
		return endDate, endDate
	case "week":
		daysSinceMonday := (int(today.Weekday()) + 6) % 7
		return today.AddDate(0, 0, -daysSinceMonday).Format("2006-01-02"), endDate
	case "month":
		return time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location()).Format("2006-01-02"), endDate
	case "last7":
		return today.AddDate(0, 0, -6).Format("2006-01-02"), endDate
	case "lastMonth":
		return today.AddDate(0, -1, 0).Format("2006-01-02"), endDate
	default:
		return "", ""
	}
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func normalizeGender(value string) string {
	value = strings.TrimSpace(value)
	switch value {
	case "男", "女":
		return value
	default:
		return ""
	}
}

func normalizeGroupName(value string) string {
	value = strings.TrimSpace(value)
	if value == "未分组" {
		return ""
	}
	return value
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
