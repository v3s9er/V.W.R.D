package crawler

import "time"

type Menu struct {
	URL       string   `json:"url"`
	Label     string   `json:"label"`
	Path      []string `json:"path"`
	ParentURL string   `json:"parent_url,omitempty"`
	Order     int      `json:"order"`
	Key       string   `json:"-"`
}

type Resource struct {
	URL      string `json:"url"`
	Path     string `json:"path,omitempty"`
	MIME     string `json:"mime,omitempty"`
	Status   int64  `json:"status,omitempty"`
	Bytes    int    `json:"bytes,omitempty"`
	Error    string `json:"error,omitempty"`
	FromMenu string `json:"from_menu,omitempty"`
}

type Page struct {
	Menu       Menu       `json:"menu"`
	FinalURL   string     `json:"final_url"`
	Title      string     `json:"title"`
	VisitedAt  time.Time  `json:"visited_at"`
	WaitResult string     `json:"wait_result"`
	HTMLPath   string     `json:"html_path,omitempty"`
	Resources  []Resource `json:"resources"`
	Error      string     `json:"error,omitempty"`
}

type Report struct {
	RootURL    string    `json:"root_url"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	DelayMS    int       `json:"min_delay_ms"`
	Menus      []Menu    `json:"menus"`
	Pages      []Page    `json:"pages"`
	Skipped    []string  `json:"skipped_menus,omitempty"`
	OutputDir  string    `json:"output_dir"`
	ZIPPath    string    `json:"zip_path"`
}

type Status struct {
	State      string    `json:"state"`
	CurrentURL string    `json:"current_url,omitempty"`
	Visited    int       `json:"visited"`
	Discovered int       `json:"discovered"`
	Saved      int       `json:"saved"`
	Message    string    `json:"message,omitempty"`
	ResultDir  string    `json:"result_dir,omitempty"`
	ZIPPath    string    `json:"zip_path,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
}
