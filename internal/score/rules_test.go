package score

import "testing"

func TestEvaluateFlagsShortAmbiguousPrompt(t *testing.T) {
	result := Evaluate("şunu düzelt")
	if result.Criteria[0].Score > 20 || len(result.Findings) != 4 || result.Score != 35 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestEvaluateRewardsConcretePrompt(t *testing.T) {
	result := Evaluate("src/parser.go dosyasındaki parseInput fonksiyonunu yalnızca boş girdi için güncelle; JSON örneği ve test ver, böylece panic oluşmasın.")
	for _, criterion := range result.Criteria {
		if criterion.Score != 100 {
			t.Fatalf("%s = %d, want 100", criterion.Name, criterion.Score)
		}
	}
}
