# Prompt senaryoları

Bu sayfa, test korpusundaki 44 promptu okunabilir biçimde gösterir. Ayrıntılı
beklentiler (`questions`, `must_keep`, `must_not_invent`) için kaynak dosya:
[cases.jsonl](cases.jsonl).

## Hata düzeltme

### `bug-ambiguous-tr` · 0–30/100

> şunu düzelt

### `bug-ambiguous-en` · 0–30/100

> fix the bug

### `bug-minimal-context` · 55–80/100

> src/parser.go içindeki parseInput fonksiyonu boş string girdisinde panic ediyor. Panic yerine hata dönsün.

### `bug-complete` · 85–100/100

> src/parser.go içindeki parseInput fonksiyonu boş string girdisinde panic ediyor. İmza değişmeden hata dönsün. Geçerli girdilerdeki mevcut davranışı koru ve boş girdi için birim test ekle.

### `bug-typos-tr` · 70–90/100

> src/parser.go daki parseInput bos girdi gelince panic atiyo bunu duzelt ama mevcut calisan durumlari bozma testte ekle

### `bug-log-only` · 20–45/100

> Ödeme isteğinde `context deadline exceeded` görüyorum.

## Özellik ve arayüz

### `feature-ambiguous` · 15–40/100

> kullanıcı profili ekle

### `feature-complete` · 85–100/100

> Kullanıcı ayarları sayfasına e-posta bildirimleri anahtarı ekle. Değer `users.notification_email_enabled` alanında saklansın ve varsayılanı true olsun. Yalnızca giriş yapmış kullanıcı kendi ayarını değiştirebilsin. API, güncellenmiş ayarı JSON olarak dönsün; birim ve entegrasyon testi ekle.

### `feature-conflicting` · 10–35/100

> Misafir kullanıcılar ayarları değiştirebilsin ama oturum açmadan hiçbir ayar değişmesin.

### `frontend-ui-complete` · 90–100/100

> Ayarlar sayfasındaki bildirim anahtarını klavye ile erişilebilir yap. Tab ve Shift+Tab ile odak sırası mantıklı olmalı; Space ve Enter anahtarı değiştirmeli; odak göstergesi görünür kalmalı. Mevcut fare davranışını koru. En az bir klavye etkileşim testi ekle.

### `frontend-ui-vague` · 5–30/100

> paneli daha güzel yap

### `terminal-ui` · 85–100/100

> Terminal içi seçim ekranında yukarı/aşağı oklarıyla iki seçenek arasında geçişi ve Enter ile seçimi ekle. Escape özgün metni koruyup editörden çıkmalı. Arayüz alternatif terminal ekranında çalışmalı; seçim tamamlandığında chat geçmişinde panel görünmemeli. Türkçe karakter girişi bozulmamalı.

### `language-mix` · 85–100/100

> Add a retry button to the Turkish error screen. The button must keep the existing `retryPayment` flow, show a loading state while the request is pending, and remain keyboard accessible. Return only the changed React component.

## Refactor ve performans

### `refactor-light` · 45–70/100

> internal/auth paketini sadeleştir ve tekrar eden token doğrulama kodunu azalt.

### `refactor-safe` · 85–100/100

> internal/auth içindeki tekrar eden JWT doğrulama kodunu ortak bir yardımcıda topla. Dışa açık fonksiyon imzalarını, hata metinlerini ve mevcut testleri değiştirme. Yeni yardımcı için birim test ekle.

### `performance-vague` · 5–30/100

> uygulamayı hızlandır

### `performance-api` · 90–100/100

> GET /search endpointinin p95 gecikmesini 800 ms'den 250 ms altına indir. Son 7 gündeki üretim sorgu dağılımını temsil eden yük testi kullan. Sonuçta önce/sonra p95, p99 ve hata oranını içeren kısa bir rapor ver. Davranışı veya sıralama mantığını değiştirme.

## Araştırma ve planlama

### `performance-video-typos` · 40–70/100

> suan sistemde hata var ui de birden fazla hata var onceki kodlardan referans almadan acik kaynak kodlardan yararlanarak sistemi arastir jetson orin nano 8 gb icin 10 kameradan gelen videoyu ayni anda isle her kamera icin 20 fps hedefle direk sona gitme fazlara bol agile ilerle her faz sonunda gorunur sonuc olsun

### `research-complete` · 90–100/100

> Mevcut kodu referans almadan sistemi yeniden tasarlamak için araştırma planı hazırla; açık kaynak çözümlerden yararlanabilirsin. Hedef platform Jetson Orin Nano 8 GB. Nihai hedef, 10 kamera akışını eşzamanlı işlemek ve her kamera için 20 FPS hedefini test edilebilir biçimde karşılamaktır. Çözümü aşamalara böl; her aşamada amaç, teknik yaklaşım, risk, doğrulama ve görünür çıktı yaz. Çıktıyı Markdown belgesi olarak sun.

### `research-source-conflict` · 60–80/100

> Önceki projedeki kodu kullanma; açık kaynak projeleri inceleyebilirsin. Bir mimari karşılaştırması hazırla.

### `hallucination-trap` · 15–45/100

> Bu uygulamanın hangi veritabanını kullandığını öğren ve migration planı hazırla.

### `multi-step-ordered` · 85–100/100

> Önce mevcut ödeme akışını ve hata metriklerini incele; sonra önerilen değişiklikleri risk sırasıyla planla; yalnızca onay aldıktan sonra kod değiştir. Her aşamada beklenen doğrulama çıktısını yaz. Çıktıyı kısa Markdown planı olarak ver.

### `multi-step-order-loss` · 65–85/100

> Önce migration planını yaz, sonra onayımı bekle, en son migration'ı uygula.

## API, güvenlik ve veri geçişi

### `api-json-complete` · 90–100/100

> POST /v1/invitations endpointi ekle. İstek gövdesi yalnızca `email` alanını kabul etsin. Başarılı yanıtta 201 ve `id`, `email`, `created_at` alanlarını döndür; geçersiz e-postada 422 dön. Var olan endpointleri değiştirme. OpenAPI şemasını ve endpoint testlerini güncelle.

### `api-underspecified` · 10–35/100

> davet API'si yap

### `security-input-validation` · 90–100/100

> POST /login için kullanıcı girdisini doğrula. E-posta ve parola alanları zorunlu olsun; doğrulama hatalarında hangi alanın geçersiz olduğunu açıklamayan genel bir 400 yanıtı dön. Parolayı loglama veya yanıta ekleme. Mevcut başarılı giriş davranışını değiştirme ve test ekle.

### `security-dangerous-ambiguity` · 10–35/100

> auth güvenliğini artır

### `migration-complete` · 90–100/100

> users tablosundaki `full_name` değerini `first_name` ve `last_name` alanlarına ayıran geri alınabilir bir migration hazırla. Boşlukla ayrıştırılamayan adları değiştirme ve raporla. Migration sırasında yazmayı durdurma. Önce staging üzerinde doğrulama adımlarını, sonra migration ve rollback komutlarını ver.

### `migration-risky` · 10–35/100

> isim alanını ikiye böl

## Test, dokümantasyon ve CLI

### `test-complete` · 85–100/100

> parseInput için tablo tabanlı birim test yaz. Boş girdi, geçerli JSON ve geçersiz JSON durumlarını kapsa. Test yalnızca mevcut genel API'yi kullansın; üretim kodunu değiştirme.

### `test-vague` · 5–30/100

> buna test yaz

### `docs-complete` · 85–100/100

> README'ye Linux'ta kurulum, `promptcheck setup-codex` kurulumu ve Ctrl+G kullanım akışını ekle. Sadece doğrulanmış komutları kullan; API anahtarı gerektirmeyen yerel çalışma ile opsiyonel model kullanımını ayır. Türkçe yaz.

### `docs-vague` · 10–35/100

> dokümantasyonu düzelt

### `command-complete` · 85–100/100

> `promptcheck --help` çıktısına `edit <dosya>` alt komutunu ekle. Açıklama, bunun EDITOR/VISUAL akışından çağrıldığını söylemeli. Mevcut komutların davranışını değiştirme ve yardım çıktısı için test ekle.

### `command-vague` · 5–30/100

> cli komutunu geliştir

## İnceleme ve içerik

### `code-review` · 85–100/100

> Bu değişikliği incele. Öncelik sırası: veri kaybı, güvenlik açığı, geriye uyumluluk ve test eksikleri. Her bulgu için dosya/satır, etki ve düzeltme önerisi yaz. Bulgu yoksa bunu açıkça belirt; kodu değiştirme.

### `code-review-vague` · 5–25/100

> koda bak

### `translation-format` · 85–100/100

> Aşağıdaki sürüm notlarını teknik Türkçeye çevir. Markdown başlıklarını ve kod bloklarını koru; ürün adlarını çevirmeden bırak. Yalnızca çeviriyi döndür.
>
> ## Fixed
> - Parser no longer crashes on empty input.

### `summarization-complete` · 80–100/100

> Aşağıdaki incident kaydını en fazla beş maddeyle özetle. Her maddede yalnızca doğrulanmış olay, etki veya sonraki adım yer alsın; neden hakkında tahmin yürütme.
>
> [incident metni burada]

### `output-format-only` · 5–35/100

> Bunu JSON yap.

## Sınırlar ve zaten iyi promptlar

### `negative-constraint` · 90–100/100

> Yalnızca `internal/score/rules.go` dosyasını değiştir. Yeni bağımlılık ekleme, puanlama API'sini değiştirme ve var olan testleri silme. Belirsiz terimlerin puan etkisini güncelle; ilgili testleri ekle.

### `negative-constraint-contradiction` · 15–40/100

> Sadece rules.go dosyasını değiştir ama yeni bir paket ekle.

### `already-good-one-paragraph` · 85–100/100

> Arama sonuçlarında kullanıcı adı ve e-posta alanlarında büyük/küçük harf duyarsız eşleşme yap. Sonuç sıralamasını değiştirme. Arama kutusuna 300 ms debounce ekle ve en az üç karakterden kısa sorgularda istek gönderme. Mevcut arama testlerini güncelle.

### `noncoding-question` · 70–95/100

> Go'da context iptal edildiğinde HTTP isteğinin ne olacağını iki kısa paragrafla açıkla.
