package modeldownloader

// WhisperModel is a model the app can fetch. RAMMB is the memory the model
// takes while transcribing, approximate: whisper.cpp's README lists the sizes
// up to medium, and large-v3-turbo (a large-v3 with a smaller decoder) is
// estimated at medium's figure.
type WhisperModel struct {
	Name       string `json:"name"`
	DownloadMB int    `json:"download_mb"`
	RAMMB      int    `json:"ram_mb"`
}

// WhisperModels lists the models offered, smallest first.
var WhisperModels = []WhisperModel{
	{Name: "tiny", DownloadMB: 75, RAMMB: 273},
	{Name: "base", DownloadMB: 142, RAMMB: 388},
	{Name: "small", DownloadMB: 466, RAMMB: 852},
	{Name: "medium", DownloadMB: 1500, RAMMB: 2100},
	{Name: "large-v3-turbo", DownloadMB: 1600, RAMMB: 2100},
	{Name: "large-v3", DownloadMB: 2900, RAMMB: 3900},
}

const gib = 1 << 30

// RecommendWhisperModel picks the most accurate model that leaves room on a
// Mac with ramBytes of memory. large-v3-turbo needs no more than medium yet
// transcribes better, so from 16 GB up there is no reason to settle for less;
// below that the model competes with the apps the user is running. An unknown
// size (0) is treated as a capable machine, since nearly every Mac sold in the
// last years is.
func RecommendWhisperModel(ramBytes int64) string {
	switch {
	case ramBytes == 0 || ramBytes >= 16*gib:
		return "large-v3-turbo"
	case ramBytes >= 8*gib:
		return "small"
	default:
		return "base"
	}
}
