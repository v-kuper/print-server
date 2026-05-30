package dailyquest

import (
	"strings"
	"testing"
	"time"
)

func TestSelectReturnsThreeDifferentCategories(t *testing.T) {
	date := time.Date(2026, 5, 30, 9, 0, 0, 0, time.FixedZone("MSK", 3*60*60))

	quests := Select(date)

	if len(quests) != 3 {
		t.Fatalf("expected 3 quests, got %d: %#v", len(quests), quests)
	}
	categories := map[Category]bool{}
	for _, quest := range quests {
		if categories[quest.Category] {
			t.Fatalf("expected different categories, got %#v", quests)
		}
		categories[quest.Category] = true
	}
}

func TestSelectAvoidsNearDuplicatesAndHighFrequencyCrowding(t *testing.T) {
	for offset := 0; offset < 370; offset++ {
		date := time.Date(2026, 1, 1+offset, 9, 0, 0, 0, time.UTC)
		quests := Select(date)
		ids := map[int]bool{}
		highFrequency := 0
		for _, quest := range quests {
			ids[quest.ID] = true
			if quest.HighFrequency {
				highFrequency++
			}
		}
		if ids[1] && ids[50] {
			t.Fatalf("expected #1 and #50 not to be selected together on %s: %#v", date.Format("2006-01-02"), quests)
		}
		if highFrequency > 1 {
			t.Fatalf("expected at most one high-frequency quest on %s, got %#v", date.Format("2006-01-02"), quests)
		}
	}
}

func TestSpendingProneQuestsHaveFreeRewrite(t *testing.T) {
	for _, id := range []int{2, 15, 18, 20, 36} {
		quest, ok := QuestByID(id)
		if !ok {
			t.Fatalf("missing quest #%d", id)
		}
		text := SafeText(quest)
		if text == quest.Text {
			t.Fatalf("expected quest #%d to have a free rewrite, got %q", id, text)
		}
		for _, forbidden := range []string{"купи", "заплати", "покуп"} {
			if strings.Contains(strings.ToLower(text), forbidden) {
				t.Fatalf("expected quest #%d rewrite to be clearly free, got %q", id, text)
			}
		}
	}
}

func TestFallbackUsesSafeText(t *testing.T) {
	quest, ok := QuestByID(2)
	if !ok {
		t.Fatal("missing quest #2")
	}

	result := Fallback([]Quest{quest})

	if len(result) != 1 || result[0].ID != 2 || result[0].Text != SafeText(quest) {
		t.Fatalf("expected fallback to use safe text, got %#v", result)
	}
}
