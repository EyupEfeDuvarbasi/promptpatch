# Prompt değerlendirme korpusu

Bu klasör, PromptPatch'in puanlama ve iyileştirme davranışını kalibre etmek için
hazırlanmış örneklerden oluşur. Örnekler, resmi model rehberlerindeki açık görev,
bağlam, kısıt ve çıktı biçimi ilkelerinden türetilmiştir:

- [Google Prompt Design Strategies](https://ai.google.dev/gemini-api/docs/prompting-strategies)
- [Google Prompt Gallery](https://ai.google.dev/gemini-api/prompts)

`cases.jsonl` dosyasında her satır bağımsız bir JSON örneğidir.

Alanlar:

- `id`: sabit örnek kimliği
- `kind`: görev türü
- `style`: promptun yazım biçimi
- `prompt`: kullanıcının girdiği özgün metin
- `expected_score`: hedef puan bandı; kesin değer değil, kalite beklentisi
- `questions`: beklenen en fazla iki soru veya boş liste
- `must_keep`: iyileştirme sırasında kaybolmaması gereken anlamlar
- `must_not_invent`: modelin varsayım yapmaması gereken alanlar

Bu korpus bir eğitim verisi değildir; promptlar hiçbir LLM'e otomatik olarak
gönderilmez. Amaç, her değişiklikte aynı örnekler üzerinde puan, soru ve anlam
koruma davranışını ölçmektir.

Kapsam: hata düzeltme, özellik, refactor, performans, araştırma/planlama,
güvenlik, API, veri geçişi, test, dokümantasyon, frontend ve terminal işleri;
ayrıca belirsiz, çelişkili, eksik, yazım hatalı ve zaten iyi promptlar.
