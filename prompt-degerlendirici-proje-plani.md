# Prompt İyileştirici & Puanlayıcı — Proje Spesifikasyonu

> Bu döküman hem projenin gerekçesini hem de Codex CLI'nin (veya başka bir kodlama ajanının) sapmadan, varsayım üretmeden inşa edebilmesi için gereken sınırları ve kararları içerir. Belirsiz/yoruma açık her nokta ya "MVP Kararı" olarak sabitlenmiş ya da açıkça "Kapsam Dışı" olarak işaretlenmiştir.

---

## 1. Amaç ve Önem

### 1.1 Problem

Terminal tabanlı AI kodlama araçları (Claude Code, Codex CLI, Copilot CLI, Gemini CLI vb.) artık yazılım geliştirme sürecinin merkezinde. Ancak bu araçların çıktı kalitesi neredeyse tamamen **kullanıcının yazdığı promptun kalitesine** bağlı. Pratikte şu sorunlar tekrar ediyor:

- Geliştiriciler çoğu zaman promptlarını aceleyle, belirsiz, eksik bağlamla yazıyor ("şunu düzelt", "bunu ekle" gibi) ve sonuç modelin yanlış varsayımlar yapmasına, gereksiz iterasyona, zaman kaybına yol açıyor.
- Prompt yazma becerisi büyük ölçüde deneyerek öğreniliyor; kimse geliştiriciye "bu prompt neden zayıf, nasıl daha iyi olurdu" diye **anlık, somut ve tarafsız** bir geri bildirim vermiyor.
- Var olan "prompt engineering" kaynakları statik, genel geçer rehberler — geliştiricinin o an, o terminalde, o spesifik promptu için işe yaramıyor.
- Bu beceri eksikliği ölçeklendikçe (bir ekip, bir organizasyon) ciddi bir verimlilik kaybına dönüşüyor: kötü promptlar → kötü/yanlış çıktı → tekrar deneme → zaman ve maliyet kaybı.

### 1.2 Neden Şimdi ve Neden Önemli

AI destekli geliştirme artık opsiyonel bir araç değil, günlük iş akışının parçası. Bu da "prompt yazma"yı git commit mesajı yazmak gibi temel bir mühendislik becerisi hâline getiriyor — ama bu beceri için hiçbir geri bildirim mekanizması, hiçbir "linter" yok. Kod için ESLint/Prettier/SonarQube gibi araçlar varken, prompt için hiçbir tarafsız kalite ölçüm aracı yok. Bu proje bu boşluğu dolduruyor.

### 1.3 Değer Önerisi

- **Geliştirici için:** Yazdığı promptu göndermeden önce (veya gönderdikten sonra) saniyeler içinde tarafsız bir puan ve somut iyileştirme önerisi alır → hem o anki işi daha verimli sonuçlanır hem de zamanla prompt yazma becerisi gelişir (farkındalık).
- **Ekip/organizasyon için:** Zamanla biriken puanlama verisi (opsiyonel, kullanıcı izniyle), ekip genelinde prompt kalitesi eğilimlerini görünür kılabilir.
- **Farklılaşma:** Var olan hiçbir araç "terminal-native, herhangi bir CLI'den bağımsız, anlık prompt puanlama" yapmıyor. Bu araç bir editör eklentisi değil, bir chatbot değil — geliştiricinin zaten yaşadığı terminal ortamına, tek bir kısayolla entegre olan, hafif bir katman.

### 1.4 Hedef Kitle

- Terminal üzerinden AI CLI araçlarıyla (Claude Code, Codex CLI vb.) günlük olarak çalışan yazılımcılar.
- Prompt yazma becerisini geliştirmek isteyen, ama bunun için ayrı bir öğrenme kaynağına vakit ayırmak istemeyen geliştiriciler.

---

## 2. Kapsam Sınırları (Codex CLI için — sapmayı önlemek amacıyla)

### 2.1 Bu Proje NE YAPAR

- Kullanıcının verdiği bir metni (prompt) tamamen local kural tabanlı olarak analiz eder ve puan üretir; LLM katmanı kullanıcı isterse ek kalite katmanı olarak çalışır.
- Aynı prompt için iyileştirilmiş bir versiyon üretir.
- Bunu terminal içinden, tek bir komut veya kısayolla tetikler.
- Sonucu terminalde okunabilir formatta gösterir.

### 2.2 Bu Proje NE YAPMAZ

- **Bir editör eklentisi DEĞİLDİR** (VS Code, JetBrains vb. entegrasyonu yok).
- **Host CLI'nin plugin sistemine bağımlı DEĞİLDİR** — Codex'in standart EDITOR/VISUAL akışını kullanır.
- **Kendi başına bir chat arayüzü / konuşma botu DEĞİLDİR** — tek işlevi puanlama + iyileştirmedir, genel sohbet yapmaz.
- **Arka planda sürekli çalışan bir daemon DEĞİLDİR** — yalnızca kullanıcı `Ctrl-G` akışını başlattığında çalışır.
- **Promptları kalıcı olarak saklamaz** — yakın sohbet bağlamı açıkça etkinleştirilirse backend'e referans olarak gönderilebilir.
- **Global OS-seviye hotkey (arka plan daemon) MVP kapsamında YOKTUR** — bkz. §5.3.
- **Genel amaçlı sohbet arayüzü DEĞİLDİR** — yalnızca prompt iyileştirme akışını destekler.

---

## 3. Mimari

**Karar: Bağımsız, platform-agnostik CLI aracı.**

Herhangi bir host AI CLI'nin (Claude Code, Copilot CLI, Codex CLI vb.) plugin sistemine bağımlı olmadan, tek başına çalışan bir binary. Kullanıcı hangi AI CLI'yi kullanıyor olursa olsun, bu araç bağımsız bir komut olarak terminalde çalışır.

**Gerekçe:** Eklenti modeli hızlı MVP sağlar ama tek ekosisteme hapseder ve her host için ayrı entegrasyon bakım yükü getirir. Bağımsız model, "hangi CLI'yi kullanırsan kullan çalışsın" hedefiyle birebir örtüşür.

**Teknoloji: Go.**

**Gerekçe:** Tek binary derleme → bağımlılıksız kurulum (kullanıcı sadece binary'yi indirir, çalıştırır). Sorunsuz kurulum/dağıtım önceliğiyle en uygun seçenek.

---

## 4. Puanlama Sistemi

**MVP Kararı: Local-first.** Kural tabanlı katman varsayılandır; LLM katmanı isteğe bağlıdır.

### 4.1 Kural Tabanlı Katman (statik analiz, LLM çağrısı gerektirmez, anlık)
Aşağıdaki heuristikleri kontrol eder ve her biri için ayrı bir alt-puan üretir:
- **Uzunluk kontrolü:** Aşırı kısa (ör. <5 kelime) promptlar otomatik düşük puan alır.
- **Belirsiz kelime tespiti:** "şunu", "bunu", "bir şekilde", "falan filan" gibi belirsizlik işaretleri içeren kelimeler taranır.
- **Bağlam eksikliği işaretleri:** Dosya adı, fonksiyon adı, teknoloji/dil belirtilmemişse işaretlenir.
- **Format/çıktı belirtimi eksikliği:** Kullanıcı ne formatta çıktı istediğini belirtmemişse işaretlenir.

Bu katman en fazla iki açıklayıcı soru seçer ve yanıtları “Amaç / Bağlam / Beklenen sonuç / Kabul kriterleri” bölümlerine derler; bilinmeyen teknik ayrıntıları uydurmaz.

### 4.2 LLM Tabanlı Katman (isteğe bağlı dinamik değerlendirme)
Kullanıcı `--model` ile özellikle etkinleştirirse, kural tabanlı katmanın tespit edemeyeceği anlamsal kaliteyi değerlendirir:
- Amacın netliği, spesifiklik, mantıksal tutarlılık, hedef/kısıt belirtimi.
- LLM'e sabit bir değerlendirme promptu ile sorulur (rubric tabanlı, serbest yorum değil) — böylece "tarafsızlık" korunur.

### 4.3 Nihai Puan
Her kriter 0-100 arası puanlanır; beş kriterin aritmetik ortalaması promptun puanıdır. Local modda yalnızca kural tabanlı puan kullanılır. LLM isteğe bağlı olarak etkinleştirilirse, her kriterde %40 kural tabanlı ve %60 LLM puanı birleştirilir.

### 4.4 Puanlama Kriterleri (MVP seti — 5 kriter, her biri 0-100 arası puanlanır)
1. **Netlik** — İstek açık ve tek anlamlı mı?
2. **Spesifiklik** — Somut detaylar (dosya, fonksiyon, teknoloji) var mı?
3. **Bağlam Yeterliliği** — Modelin ihtiyaç duyacağı arka plan bilgisi verilmiş mi?
4. **Kısıt/Format Belirtimi** — Beklenen çıktı formatı, kısıtlar belirtilmiş mi?
5. **Amaç Açıklığı** — "Neden" bu isteğin yapıldığı anlaşılıyor mu (gerekirse)?

> Not: Bu 5 kriter MVP başlangıç noktasıdır, sabit/nihai değildir — geliştirme sürecinde kalibre edilebilir.

---

## 5. Kullanım Akışı ve Komut Arayüzü

### 5.1 Editör komutu

```
promptcheck edit <dosya>
```

Bu komut Codex'in `EDITOR`/`VISUAL` akışından çağrılır.

### 5.2 Kullanıcı akışı

Prompt editöre aktarılır, en fazla iki karar değiştirici soru sorulur, özgün ve
iyileştirilmiş sürüm karşılaştırılır; kullanıcı seçerse dosyaya yazılır.

### 5.3 Tetikleme (Kısayol)

**MVP Kararı: Shell-binding veya host CLI'nin standart editör akışı (global OS hotkey DEĞİL).**

Kurulum sırasında kullanıcının `.bashrc`/`.zshrc` dosyasına eklenen bir shell fonksiyonu/keybinding aracılığıyla çalışabilir. Host CLI standart `EDITOR`/`VISUAL` akışını destekliyorsa PromptPatch bu editör olarak da çalışır: host'un verdiği mevcut taslak metni aynı terminalde analiz eder, en fazla iki soru sorar ve kullanıcının seçtiği sürümü aynı dosyaya geri yazar. Örneğin Codex CLI'nin `Ctrl-G` prompt editörü bu akışı kullanır. Global, arka planda çalışan bir OS-seviye daemon **MVP kapsamında yoktur** — bu, kurulum karmaşıklığını ve platformlar arası (Linux/macOS/Windows) farklı API gereksinimlerini MVP'den çıkarmak için bilinçli bir sınırlamadır.

### 5.4 Örnek Çıktı (Kısa Mod)

```
$ promptcheck "şu fonksiyonu düzelt"
Puan: 3/10 — Belirsiz istek: hangi fonksiyon, ne tür bir hata, hangi dosya belirtilmemiş.
Detay için: promptcheck -d "şu fonksiyonu düzelt"
```

### 5.5 Örnek Çıktı (Detaylı Mod)

```
$ promptcheck -d "şu fonksiyonu düzelt"
Puan: 3/10

Kriter Kırılımı:
  Netlik:              2/10  — "şu fonksiyon" hangi fonksiyon belli değil
  Spesifiklik:          2/10  — Dosya/fonksiyon adı, dil belirtilmemiş
  Bağlam Yeterliliği:   3/10  — Hata mesajı veya beklenen davranış yok
  Kısıt/Format:         4/10  — Çıktı formatı belirtilmemiş ama basit bir düzeltme isteği
  Amaç Açıklığı:        4/10  — "düzelt" kelimesinden amaç kısmen anlaşılıyor

İyileştirilmiş Prompt:
"src/utils/parser.go dosyasındaki parseInput fonksiyonu boş string
girdiğinde panic ediyor. Bu durumda panic yerine boş bir sonuç ve
nil hata dönmesini istiyorum. Fonksiyonu buna göre güncelle."

(Not: Yukarıdaki örnek, kullanıcının gerçek bağlamını bilmediğimiz için
şablon amaçlıdır — gerçek sistemde LLM, kullanıcıdan aldığı sınırlı
bilgiyle en olası iyileştirmeyi üretir.)
```

---

## 6. Opsiyonel API Key Yönetimi

**MVP Kararı: Sadece kullanıcı LLM modunu (`--model`) seçerse anahtar yönetimine gir.** Local mod hiçbir anahtar istemez.

1. LLM modu ilk çalıştırıldığında kullanıcı varsayılan sağlayıcıyı seçer. Araç seçilen sağlayıcının ortam değişkenini kontrol eder:
   - `ANTHROPIC_API_KEY` ortam değişkeni
   - `OPENAI_API_KEY` ortam değişkeni (Codex CLI kullanıcıları için)
   - `GEMINI_API_KEY` ortam değişkeni
2. Ortamda anahtar yoksa kullanıcıdan interaktif olarak key ister ve `~/.config/promptcheck/config.yaml` içine kaydeder. Ortamdan bulunan anahtar yeniden dosyaya yazılmaz.
3. Bilinen başka CLI config dosyaları MVP'de okunmaz; kesin yollar araştırılması gereken açık konudur (§9).
3. **Güvenlik notu:** Başka bir aracın config/key dosyasını okumadan önce kullanıcıya açıkça bildirim yapılır ve onay istenir — sessizce başka bir aracın kimlik bilgilerini okumak kabul edilemez.

---

## 7. Gizlilik ve Loglama

**MVP Kararı: Tamamen local, hiçbir veri dışarı gönderilmez (LLM API çağrısı hariç).**

- Promptlar ve puanlar hiçbir merkezi sunucuya loglanmaz.
- Tek dışarı giden veri, puanlama/iyileştirme için LLM API'sine (kullanıcının kendi key'iyle) gönderilen prompt metnidir — bu zaten kullanıcının bilinçli kararıdır.
- Telemetri/istatistik toplama **MVP'de yoktur**. İleride eklenirse kesinlikle **opt-in** (varsayılan kapalı) olmalı ve ne toplandığı açıkça belirtilmelidir.

---

## 8. Dağıtım

**MVP Kararı:** GitHub Releases üzerinden platform bazlı binary indirme (Linux, macOS) + `go install` desteği. Homebrew formula gibi ek kanallar MVP sonrası.

---

## 9. Açık Konular (MVP Sonrası Netleştirilecek — Kapsam Dışı Değil, Sadece Ertelenmiş)

Bunlar Codex CLI'nin MVP'yi geliştirirken **karar vermesi gereken değil**, ileride ayrı bir çalışmayla netleştirilecek konulardır:

1. Hangi CLI'ların key/config dosyalarının hangi yollarda tutulduğuna dair kesin bir araştırma (adapter listesi).
2. Puanlama kriterleri ağırlıklarının kalibrasyonu (gerçek kullanım verisiyle).
3. Global hotkey (OS-seviye) desteğinin v2'de eklenip eklenmeyeceği.
4. Windows desteği.
5. Proje adı (şu an "promptcheck" örnek/geçici isim olarak kullanılmıştır, nihai değildir).
6. Telemetri eklenip eklenmeyeceği ve nasıl.

---

## 10. Codex CLI için Uygulama Talimatı Özeti

> Bu bölüm, kodlama ajanının işe başlarken izlemesi gereken sınırları tek yerde özetler.

- Sadece §2.1'de tanımlanan işlevleri geliştir. §2.2'de "ne yapmaz" olarak işaretlenenlere **girişme**.
- §9'daki "Açık Konular" için kendi başına karar üretme — bunlar MVP kapsamı dışıdır, dokunma.
- Karar tabloları (§3, §4, §5, §6, §7, §8) MVP için **sabit kabul edilir**; belirsizlik hissedersen buradaki kararlara sadık kal, alternatif mimari/teknoloji önerme.
- Şüpheli/belirsiz bir noktayla karşılaşırsan, kod yazmadan önce kullanıcıya sor — varsayım üretip ilerleme.
