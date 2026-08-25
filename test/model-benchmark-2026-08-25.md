# Codex model karşılaştırması — 25 Ağustos 2026

## Yöntem

Üç yapılandırma, `model-cases.jsonl` içindeki 20 `core` vakada aynı yeniden
yazma rubriği ve JSON şemasıyla çalıştırıldı. Araçlar salt-okunur, geçici bir
dizinde ve kaydedilmeyen Codex oturumlarında kullanıldı. Boş/yarım çıktı,
somut bilgi kaybı ve kaynakta olmayan sayı otomatik geçiş kapısı; niyet koruma,
gereksiz ekleme, gramer ve sadelik manuel inceleme ölçütüydü.

| Yapılandırma | Geçiş | Medyan | Çıktı tokenı | Reasoning tokenı | Giriş / cached |
|---|---:|---:|---:|---:|---:|
| Terra / low | 20/20 | 8.28 sn | 3.240 | 0 | 290.027 / 164.864 |
| Terra / medium | 20/20 | 7.77 sn | 3.055 | 0 | 290.027 / 152.832 |
| Sol / low | 20/20 | 11.65 sn | 5.876 | 1.337 | 319.727 / 70.400 |

Terra/medium, çekirdek sette en kısa medyan süreyi ve en düşük çıktı tokenını
verdi. Sol bazı karmaşık vakalarda daha ayrıntılıydı; ancak çıktıları yaklaşık
iki kat uzundu, iki vakada ek ajan turu kullandı ve çoğu vakada talep edilmeyen
teslimat açıklamaları ekledi. Prompt editörü için bu fark ek maliyet ve
gecikmeyi karşılamadı.

## Geniş korpus sonucu

Terra/medium kalan 40 vakada otomatik geçiş kapısını 40/40 geçti. Çekirdek ve
geniş set birlikte 60 vaka için medyan süre 8.04 saniyeydi. Toplam kullanım
870.707 giriş, 669.440 cached giriş, 9.010 çıktı ve 14 reasoning tokenı oldu.

Manuel inceleme altı dil/niyet kusuru buldu. Rubriğe yeni görev eklememe, süre
koşulunu doğru özneye bağlama, tek emir kipi ve tekrar/gramer kontrolü eklendi.
Hedefli tekrar testinde bunların dördü düzeldi; iki vaka kaldı:

- `performance-streaming-memory`: iki emir arasındaki noktalama eksik kaldı.
- `privacy-core`: yalnızca e-posta ve telefon denirken kapsamı “diğer PII” ile
  genişletti.

Sonuç: varsayılan aday `gpt-5.6-terra` + `medium`. Üretime bağlamadan önce bu
iki regresyon kalite kapısına eklenmeli; bütün seti yeniden çalıştırmak yerine
hedefli vaka komutu kullanılabilir:

```bash
go run ./tools/promptbench -tier all \
  -ids performance-streaming-memory,privacy-core \
  -model gpt-5.6-terra -reasoning medium
```
