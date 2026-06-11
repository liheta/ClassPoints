package app

import "time"

type Class struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Teacher   string    `json:"teacher"`
	CreatedAt time.Time `json:"createdAt"`
}

type Student struct {
	ID        int64     `json:"id"`
	ClassID   int64     `json:"classId"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	Gender    string    `json:"gender"`
	CreatedAt time.Time `json:"createdAt"`
	Score     int       `json:"score"`
}

type Rule struct {
	ID        int64     `json:"id"`
	ClassID   int64     `json:"classId"`
	Name      string    `json:"name"`
	Score     int       `json:"score"`
	Category  string    `json:"category"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
}

type ScoreRecord struct {
	ID           int64      `json:"id"`
	ClassID      int64      `json:"classId"`
	StudentID    int64      `json:"studentId"`
	StudentName  string     `json:"studentName"`
	Gender       string     `json:"gender"`
	RuleID       *int64     `json:"ruleId"`
	RuleName     string     `json:"ruleName"`
	Score        int        `json:"score"`
	Reason       string     `json:"reason"`
	Note         string     `json:"note"`
	CreatedAt    time.Time  `json:"createdAt"`
	UndoneAt     *time.Time `json:"undoneAt"`
	UndoReason   string     `json:"undoReason"`
	Effective    bool       `json:"effective"`
	EffectiveTxt string     `json:"effectiveText,omitempty"`
}

type Settings struct {
	SchoolName      string `json:"schoolName"`
	BackupDir       string `json:"backupDir"`
	AutoBackupMins  int    `json:"autoBackupMins"`
	BackupKeepCount int    `json:"backupKeepCount"`
}

type Dashboard struct {
	Class         Class         `json:"class"`
	StudentCount  int           `json:"studentCount"`
	TotalScore    int           `json:"totalScore"`
	TodayRecords  int           `json:"todayRecords"`
	Leader        *Student      `json:"leader"`
	RecentRecords []ScoreRecord `json:"recentRecords"`
	Ranking       []Student     `json:"ranking"`
}
