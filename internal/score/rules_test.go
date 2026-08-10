package score

import "testing"

func TestEvaluateFlagsShortAmbiguousPrompt(t *testing.T) {
	result := Evaluate("şunu düzelt")
	if result.Criteria[0].Score > 40 || !result.NeedsContext || !result.NeedsFormat || !result.NeedsClarifying || result.Score >= 30 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestEvaluateRewardsConcretePrompt(t *testing.T) {
	result := Evaluate("src/parser.go dosyasındaki parseInput fonksiyonunu yalnızca boş girdi için güncelle; JSON örneği ve test ver, böylece panic oluşmasın.")
	if result.Score < 90 || result.NeedsContext || result.NeedsFormat || result.NeedsClarifying {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestEvaluatePenalizesMissingExpectedResult(t *testing.T) {
	result := Evaluate("src/parser.go içindeki parseInput fonksiyonunu güncelle.")
	if result.Criteria[2].Score >= 65 || !result.NeedsFormat {
		t.Fatalf("expected-result score: %#v", result)
	}
}

func TestEvaluateRecognizesNaturalExpectedResult(t *testing.T) {
	result := Evaluate("src/parser.go dosyasında boş girdi alındığında hata dönmesini sağlayın.")
	if result.Criteria[2].Score < 65 || result.Criteria[0].Score < 70 {
		t.Fatalf("natural prompt was underscored: %#v", result)
	}
}
