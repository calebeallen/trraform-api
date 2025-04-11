package schemas

type LeaderboardEntry struct {
	PlotId string  `json:"id"`
	Votes  float64 `json:"votes"`
	Dir    int     `json:"dir"`
}
