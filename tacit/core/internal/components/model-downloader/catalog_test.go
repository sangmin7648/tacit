package modeldownloader

import "testing"

func TestRecommendWhisperModel(t *testing.T) {
	cases := []struct {
		gb   int64
		want string
	}{{4, "base"}, {8, "small"}, {12, "small"}, {16, "large-v3-turbo"}, {32, "large-v3-turbo"}, {0, "large-v3-turbo"}}
	for _, c := range cases {
		if got := RecommendWhisperModel(c.gb * gib); got != c.want {
			t.Errorf("%d GB: got %s, want %s", c.gb, got, c.want)
		}
	}
}

func TestRecommendedModelIsOffered(t *testing.T) {
	for _, gb := range []int64{0, 4, 8, 16} {
		name := RecommendWhisperModel(gb * gib)
		found := false
		for _, m := range WhisperModels {
			found = found || m.Name == name
		}
		if !found {
			t.Errorf("%s is recommended but not in WhisperModels", name)
		}
	}
}
