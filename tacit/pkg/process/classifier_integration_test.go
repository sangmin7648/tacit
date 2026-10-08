//go:build integration

package process

import (
	"context"
	"strings"
	"testing"
)

type classifierTestCase struct {
	name           string
	text           string
	expectSkip     bool
	expectCategory string // soft check: category top-level should contain this keyword
}

var singleTestCases = []classifierTestCase{
	{
		name:           "일상_점심",
		text:           "오늘 점심 뭐 먹지? 김치찌개 먹을까 아니면 그냥 편의점 갈까",
		expectSkip:     false,
		expectCategory: "daily",
	},
	{
		name: "개발_goroutine",
		text: "Go에서 goroutine leak 방지하려면 context로 cancel 전파해야 해. " +
			"defer cancel() 꼭 넣어야 되고 done 채널 닫는 패턴도 같이 써야 안전함",
		expectSkip:     false,
		expectCategory: "dev",
	},
	{
		name: "회의_스프린트",
		text: "다음 주 스프린트 목표는 결제 모듈 완성이야. " +
			"API 설계는 내가 담당하고 프론트엔드 연동은 김대리한테 부탁하기로 했어. " +
			"목요일까지 PR 올려야 됨",
		expectSkip:     false,
		expectCategory: "work",
	},
	{
		name:           "건강_운동",
		text:           "오늘 헬스장에서 스쿼트 5세트 했는데 허벅지가 터질 것 같아. 내일 못 걸을 듯",
		expectSkip:     false,
		expectCategory: "health",
	},
	{
		name: "요리_레시피",
		text: "제육볶음 만들 때 돼지고기 앞다리살 써야 맛있어. " +
			"고추장이랑 간장 비율이 2대1이고 설탕 약간 넣으면 됨. " +
			"마늘은 많이 넣을수록 좋고 참기름은 마지막에",
		expectSkip:     false,
		expectCategory: "lifestyle",
	},
	{
		name: "알고리즘_공부",
		text: "오늘 알고리즘 공부했는데 다익스트라 알고리즘 드디어 이해했어. " +
			"우선순위 큐를 쓰면 시간복잡도가 O(E log V)가 되는 거고, " +
			"BFS랑 다르게 가중치 있는 그래프에서 최단경로 찾을 때 씀. " +
			"음수 가중치는 벨만-포드 써야 하고, 다익스트라는 음수 있으면 틀림",
		expectSkip:     false,
		expectCategory: "learning",
	},
	{
		name:           "감정_일기",
		text:           "오늘 발표 완전 망했다. 준비를 너무 못했나봐. 다음엔 더 잘 할 수 있겠지",
		expectSkip:     false,
		expectCategory: "",
	},
	{
		name:       "노이즈",
		text:       "음... 어... 그... 음... 어...",
		expectSkip: true,
	},
}

// VAD split test cases - same conversation split into 2 segments
var vadSplitCases = []classifierTestCase{
	{
		name: "독서메모_part1",
		text: "방금 읽은 책에서 좋은 구절 봤어. 칼 뉴포트의 딥 워크인데",
		expectSkip: false,
	},
	{
		name: "독서메모_part2",
		text: "집중력이 곧 경쟁력이라고 했는데, 요즘 내가 너무 산만하게 일하는 것 같아서 반성하게 됐어. " +
			"포모도로 기법이라도 써봐야 할 것 같음",
		expectSkip: false,
	},
}

func newTestClassifier(t *testing.T) Classifier {
	t.Helper()
	return NewClaudeClassifier("")
}

func TestClassifier_Classify_Integration(t *testing.T) {
	classifier := newTestClassifier(t)
	ctx := context.Background()

	for _, tc := range singleTestCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := classifier.Classify(ctx, tc.text, nil)
			if err != nil {
				t.Fatalf("Classify() error: %v", err)
			}

			if tc.expectSkip {
				if !result.Skip {
					t.Errorf("expected skip=true, got result: title=%q category=%q summary=%q",
						result.Title, result.Category, result.Summary)
				}
				return
			}

			if result.Skip {
				t.Errorf("unexpected skip=true for non-noise input")
				return
			}
			if result.Title == "" {
				t.Errorf("Title is empty")
			}
			if result.Summary == "" {
				t.Errorf("Summary is empty")
			}
			if result.Category == "" {
				t.Errorf("Category is empty")
			}

			t.Logf("title=%q  category=%q  summary=%q", result.Title, result.Category, result.Summary)

			if tc.expectCategory != "" && !strings.Contains(result.Category, tc.expectCategory) {
				t.Logf("SOFT WARN: expected category to contain %q, got %q", tc.expectCategory, result.Category)
			}
		})
	}
}

// TestClassifier_ClassifyBatch_VADSplit tests that VAD-split segments of the same
// conversation are classified into the same top-level category when batched.
func TestClassifier_ClassifyBatch_VADSplit(t *testing.T) {
	classifier := newTestClassifier(t)
	ctx := context.Background()

	texts := make([]string, len(vadSplitCases))
	for i, tc := range vadSplitCases {
		texts[i] = tc.text
	}

	results, err := classifier.ClassifyBatch(ctx, texts, nil)
	if err != nil {
		t.Fatalf("ClassifyBatch() error: %v", err)
	}
	if len(results) != len(texts) {
		t.Fatalf("expected %d results, got %d", len(texts), len(results))
	}

	for i, result := range results {
		tc := vadSplitCases[i]
		if result.Skip {
			t.Errorf("[%s] unexpected skip=true", tc.name)
			continue
		}
		t.Logf("[%s] title=%q  category=%q  summary=%q", tc.name, result.Title, result.Category, result.Summary)
	}

	// Soft check: both parts should share the same top-level category
	if len(results) >= 2 && !results[0].Skip && !results[1].Skip {
		cat0 := strings.SplitN(results[0].Category, "/", 2)[0]
		cat1 := strings.SplitN(results[1].Category, "/", 2)[0]
		if cat0 != cat1 {
			t.Logf("SOFT WARN: VAD-split parts have different top-level categories: %q vs %q", cat0, cat1)
		}
	}
}
