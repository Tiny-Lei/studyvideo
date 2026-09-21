package store

import "time"

type Category struct {
	ID          int64     `json:"id"`
	TopicID     int64     `json:"topic_id"`
	TopicName   string    `json:"topic_name,omitempty"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Sort        int       `json:"sort"`
	VideoCount  int       `json:"video_count"`
	FailCount   int       `json:"fail_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Topic struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Sort          int       `json:"sort"`
	VideoCount    int       `json:"video_count"`
	FailCount     int       `json:"fail_count"`
	CategoryCount int       `json:"category_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type VideoStats struct {
	VisitUV int64 `json:"visit_uv"`
	WatchUV int64 `json:"watch_uv"`
	VisitPV int64 `json:"visit_pv"`
	WatchPV int64 `json:"watch_pv"`
}

type Video struct {
	ID           int64      `json:"id"`
	TopicID      int64      `json:"topic_id"`
	TopicName    string     `json:"topic_name,omitempty"`
	CategoryID   int64      `json:"category_id"`
	CategoryName string     `json:"category_name,omitempty"`
	Title        string     `json:"title"`
	URL          string     `json:"url,omitempty"`
	Description  string     `json:"description"`
	Tags         string     `json:"tags"`
	Sort         int        `json:"sort"`
	Status       string     `json:"status"`
	StatusCode   int        `json:"status_code"`
	LatencyMS    int        `json:"latency_ms"`
	ErrorMsg     string     `json:"error_msg,omitempty"`
	CheckedAt    *time.Time `json:"checked_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	// 访问 / 观看统计（仅管理端列表填充）
	Stats VideoStats `json:"stats"`
}

type PDF struct {
	ID        int64     `json:"id"`
	VideoID   int64     `json:"video_id"`
	Title     string    `json:"title"`
	FilePath  string    `json:"file_path,omitempty"`
	FileName  string    `json:"file_name"`
	FileSize  int64     `json:"file_size"`
	MimeType  string    `json:"mime_type"`
	Sort      int       `json:"sort"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BlockedIP struct {
	IP           string    `json:"ip"`
	Reason       string    `json:"reason"`
	BlockedUntil time.Time `json:"blocked_until"`
	CreatedAt    time.Time `json:"created_at"`
}

type Stats struct {
	Topics      int64 `json:"topics"`
	Categories  int64 `json:"categories"`
	Videos      int64 `json:"videos"`
	FailVideos  int64 `json:"fail_videos"`
	Unchecked   int64 `json:"unchecked"`
	BlockedIPs  int64 `json:"blocked_ips"`
	TodayPlays  int64 `json:"today_plays"`
	TodayVisits int64 `json:"today_visits"`
}

type VideoFilter struct {
	TopicID    int64
	CategoryID int64
	Keyword    string
	Status     string
	Page       int
	PageSize   int
}

type MaterialCategory struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Tags          string    `json:"tags"`
	Sort          int       `json:"sort"`
	MaterialCount int       `json:"material_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Material struct {
	ID           int64     `json:"id"`
	CategoryID   int64     `json:"category_id"`
	CategoryName string    `json:"category_name,omitempty"`
	GroupName    string    `json:"group_name"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Tags         string    `json:"tags"`
	FilePath     string    `json:"file_path,omitempty"`
	FileName     string    `json:"file_name"`
	FileSize     int64     `json:"file_size"`
	MimeType     string    `json:"mime_type"`
	FileExt      string    `json:"file_ext"`
	Sort         int       `json:"sort"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type MaterialFilter struct {
	CategoryID int64
	GroupName  string
	Keyword    string
	FileExt    string
	Page       int
	PageSize   int
}
