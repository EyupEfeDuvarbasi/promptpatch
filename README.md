# promptpatch

Codex içinde `Ctrl-G` ile AI kodlama promptlarını merkezi PromptPatch server veya yerel model üzerinden iyileştirir.

## Kurulum

```sh
go install github.com/EyupEfeDuvarbasi/promptpatch/cmd/promptcheck@main
```

Alternatif olarak GitHub Releases sayfasından platformunuza uygun binary'yi indirin ve PATH içindeki bir dizine koyun.

Windows'ta Go kurulu değilse:

```powershell
winget install GoLang.Go
```

Kurulumdan sonra yeni bir PowerShell açıp çalıştırabilirsiniz:

```powershell
go install github.com/EyupEfeDuvarbasi/promptpatch/cmd/promptcheck@main
promptcheck --help
```

Yeni bilgisayar kurulumu, local/remote test akışı ve sorun giderme için:
[Yeni bilgisayarda kurulum ve test](docs/new-computer-setup.md)

## 10 prompt ile hızlı doğrulama

Kaynak kodundan çalıştırırken 10 örnek promptu ve bağlamlı prompt davranışını
görmek için:

```powershell
go run ./tools/prompttest
```

Bu test sentetik cevaplar kullanır; gerçek model kalitesini değil, soru üretimi,
bağlam kullanımı ve yerel fallback akışını doğrular. Kaynak kodu testleri için:

```powershell
go test ./...
go vet ./...
```

Windows'ta Go ve PromptPatch kurulumu, Codex entegrasyonu ve local fallback ayarı
tek script ile yapılabilir:

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\install-promptpatch.ps1
```

Bu script Codex CLI'nin zaten kurulu ve PATH içinde olduğunu varsayar. Remote
server kullanmak için:

```powershell
.\install-promptpatch.ps1 -ServerUrl "https://promptpatch.example.com" -ServerToken "uzun-rastgele-token"
```

Ollama ve önerilen modeli de kurmak için:

```powershell
.\install-promptpatch.ps1 -InstallOllama
```

## Codex'te `Ctrl-G`

Codex CLI, `Ctrl-G` ile o an yazdığınız promptu editöre aktarır. Bir kez aşağıdaki kurulumu yapın ve **yeni bir terminalde Codex'i normal şekilde açın**:

```powershell
promptcheck setup-codex
```

`Ctrl-G` artık aynı terminalde PromptPatch ekranını açar. Taslak prompt otomatik gelir, ekran yatay kaydırma gerektirmeden satır kırar; eksik bilgi varsa en fazla iki karar değiştirici soru sorar. İlk kurulumda yakın sohbet bağlamı için kapalı, 800, 2.000 veya 4.000 kelimelik sınır seçilir (varsayılan 2.000). Remote server seçilirse sohbet bağlamının gönderimi için ayrıca onay istenir; onay yoksa yalnız taslak prompt gönderilir.

Yanıtlar önce yerel eksik-bilgi kontrolüyle toplanır; sonra yapılandırmaya göre merkezi PromptPatch server'a veya yerel modele verilir. Model gerçek iyileştirilmiş promptu üretir, iki sürüm yerel kurallarla puanlanır. Soru-cevap metnini sona ekleyen, somut gereksinimleri kaybeden veya skoru düşüren model çıktıları kabul edilmez. `↑`/`↓` ve `Enter` ile sürümü seçersiniz.

Bu ayar yalnızca `codex` komutunu saran bir shell/PowerShell fonksiyonu ekler; başka programların editör tercihini değiştirmez. Windows'ta PowerShell 7 ve Windows PowerShell profil dosyalarına fonksiyon eklenir; ayrıca `%LOCALAPPDATA%\PromptPatch\bin\promptpatch-codex-editor.cmd` ve `%LOCALAPPDATA%\PromptPatch\bin\promptpatch-codex.cmd` wrapper'ları oluşturulur. Etkinleşmesi için yeni PowerShell penceresi açın.

Windows'ta `Ctrl-G`, PowerShell kısayolu değil Codex terminal arayüzünün kendi kısayoludur. Codex'i PromptPatch ayarlarıyla açmak için yeni PowerShell'de normal `codex` komutunu kullanın. Profil fonksiyonu yüklenmezse aynı işi doğrudan şu launcher yapar:

```powershell
& "$env:LOCALAPPDATA\PromptPatch\bin\promptpatch-codex.cmd"
```

Bu launcher `VISUAL` ve `EDITOR` değişkenlerini PromptPatch editör wrapper'ına ayarlayıp gerçek `codex.exe` komutunu başlatır. `Ctrl-G` yine Codex ekranı odaktayken çalışır.

Yeni bir kullanıcı cihazında Ollama kurmadan merkezi server kullanmak için `promptcheck setup-codex` sırasında server sorusuna `y` yanıtı verip API URL'sini girin. Token yalnız ortam değişkeninden okunur:

```powershell
$env:PROMPTPATCH_API_URL = "https://promptpatch.example.com"
$env:PROMPTPATCH_API_TOKEN = "uzun-rastgele-token"
codex
```

## Kullanım

```powershell
codex
```

Codex ekranında prompt yazarken `Ctrl-G` kullanın. Doğrudan `promptcheck "<prompt>"` kullanımı kaldırılmıştır; PromptPatch kullanıcı akışı EDITOR/VISUAL entegrasyonu üzerinden çalışır.

Yakın sohbet bağlamını sonradan açmak veya değiştirmek için:

```powershell
promptcheck configure-context
```

## Server modu

Kullanıcılara Ollama kurdurmadan çalıştırmak için PromptPatch'i kendi server'ınızda HTTP API olarak başlatabilirsiniz. Bu mimaride dışarıya yalnızca PromptPatch API açılır; Ollama `127.0.0.1:11434` veya private network üzerinde kalır.

```text
Kullanıcı / istemci
  -> PromptPatch HTTP API
  -> private Ollama
  -> gemma3:4b
```

Server kurulumu için önerilen model:

```sh
ollama pull gemma3:4b
```

API sunucusunu başlatma:

```sh
export PROMPTPATCH_SERVER_ADDR=127.0.0.1:8080
export PROMPTPATCH_SERVER_TOKEN="uzun-rastgele-token"
export PROMPTPATCH_OLLAMA_URL=http://127.0.0.1:11434/api/generate
export PROMPTPATCH_OLLAMA_MODEL=gemma3:4b
export PROMPTPATCH_MAX_CONCURRENCY=2
export PROMPTPATCH_RATE_LIMIT_PER_MINUTE=10
promptcheck serve
```

İstek örneği:

```sh
curl -X POST http://127.0.0.1:8080/v1/improve \
  -H "Authorization: Bearer uzun-rastgele-token" \
  -H "Content-Type: application/json" \
  -d '{"prompt":"src/parser.go dosyasındaki boş girdi hatasını düzelt"}'
```

Yanıtta `source` alanı `ollama` ise model çıktısı kullanılmıştır. Ollama çalışmıyorsa, timeout olursa veya model güvenilir çıktı üretmezse API hata döndürmek yerine `source: "local"` ile yerel kural tabanlı iyileştirmeye düşer.

Hazır endpointler:

- `GET /healthz`: proses ayakta mı kontrol eder.
- `GET /readyz`: Ollama'ya ve yüklü modele erişilebiliyor mu kontrol eder.
- `POST /v1/improve`: promptu puanlar ve iyileştirilmiş prompt döndürür.
- `GET /metrics`: prompt veya token içermeyen Prometheus sayaçlarını döndürür.

## GitHub'dan server'a yükleme

Ubuntu/Debian tabanlı bir server için örnek kurulum:

```sh
sudo apt-get update
sudo apt-get install -y curl git
curl -fsSL https://ollama.com/install.sh | sh
ollama pull gemma3:4b
```

Go kuruluysa binary'yi doğrudan GitHub'dan kurabilirsiniz:

```sh
go install github.com/EyupEfeDuvarbasi/promptpatch/cmd/promptcheck@main
sudo install -m 0755 "$(go env GOPATH)/bin/promptcheck" /usr/local/bin/promptcheck
```

Alternatif olarak repoyu klonlayıp build alın:

```sh
git clone https://github.com/EyupEfeDuvarbasi/promptpatch.git
cd promptpatch
go build -o promptcheck ./cmd/promptcheck
sudo install -m 0755 promptcheck /usr/local/bin/promptcheck
```

Önce yalnız root'un okuyabildiği sunucu ayar dosyasını oluşturun. Token için
yer tutucu kullanmayın; `openssl rand -hex 32` ile üretilmiş gerçek değeri yazın.

```sh
sudo install -d -m 0700 /etc/promptpatch
sudo tee /etc/promptpatch/promptpatch.env >/dev/null <<'EOF'
PROMPTPATCH_SERVER_ADDR=127.0.0.1:8080
PROMPTPATCH_SERVER_TOKEN=uzun-rastgele-token
PROMPTPATCH_OLLAMA_URL=http://127.0.0.1:11434/api/generate
PROMPTPATCH_OLLAMA_MODEL=gemma3:4b
PROMPTPATCH_MAX_CONCURRENCY=2
PROMPTPATCH_RATE_LIMIT_PER_MINUTE=10
EOF
sudo chmod 600 /etc/promptpatch/promptpatch.env
```

Ardından systemd servisini oluşturun:

```sh
sudo tee /etc/systemd/system/promptpatch.service >/dev/null <<'EOF'
[Unit]
Description=PromptPatch API
After=network-online.target ollama.service
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/promptpatch/promptpatch.env
ExecStart=/usr/local/bin/promptcheck serve
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
```

Servisi etkinleştirme:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now promptpatch
sudo systemctl status promptpatch
```

Production notları:

- Ollama portu `11434` internete açılmamalı.
- Public domain için Nginx/Caddy ile yalnızca PromptPatch API'ye reverse proxy verin.
- `PROMPTPATCH_SERVER_TOKEN` public bind için zorunludur ve uzun rastgele değer olmalıdır. Tokensız kullanım yalnızca `127.0.0.1` üzerinde mümkündür. İstemcide token yalnız `PROMPTPATCH_API_TOKEN` ortam değişkeninden okunur; config dosyasına yazılmaz.
- GPU gücüne göre `PROMPTPATCH_MAX_CONCURRENCY` değerini düşük başlatın; küçük GPU/CPU için `1`, orta GPU için `2-4`.
- Deploy sonrası `curl http://127.0.0.1:8080/readyz` ile model erişimini kontrol edin.

## Yeni kullanıcı cihazı entegrasyonu

Bu akışta kullanıcının bilgisayarına Ollama veya model kurulmaz; yalnızca PromptPatch CLI ve Codex wrapper kurulur.

1. Go kurulu değilse kurun:

```powershell
winget install GoLang.Go
```

2. PromptPatch'i GitHub'dan kurun:

```powershell
go install github.com/EyupEfeDuvarbasi/promptpatch/cmd/promptcheck@main
promptcheck --help
```

3. Codex entegrasyonunu kurun:

```powershell
promptcheck setup-codex
```

Kurulum sorularında:

- Yakın sohbet bağlamı için varsayılan `3` seçilebilir.
- Merkezi server sorusunda `y` seçin.
- Server URL olarak public reverse proxy adresini girin, örnek: `https://promptpatch.example.com`.
- `PROMPTPATCH_API_TOKEN` değerini server'daki `PROMPTPATCH_SERVER_TOKEN` ile aynı olacak şekilde ortam değişkeni olarak ayarlayın.

4. Yeni PowerShell penceresi açın ve Codex'i wrapper üzerinden başlatın:

```powershell
codex
```

`codex.exe` yazmayın; bu PowerShell fonksiyonunu atlayabilir. Doğrulama:

```powershell
Get-Command codex -All
```

İlk sonuç `Function codex` olmalıdır. Profil yüklenmediyse doğrudan launcher kullanılabilir:

```powershell
& "$env:LOCALAPPDATA\PromptPatch\bin\promptpatch-codex.cmd"
```

5. Codex içinde prompt yazarken `Ctrl-G` kullanın. Remote server erişilemezse veya model güvenilir çıktı üretmezse PromptPatch yerel kural tabanlı iyileştirmeye düşer; kullanıcı akışı bozulmaz.

## Model ve gizlilik

Editör akışı öncelikle yapılandırılmış PromptPatch server'ı, sonra yerel Ollama'yı,
son olarak deterministik yerel iyileştirmeyi dener. Doğrudan `--model` CLI akışı
desteklenmez.

Yakın sohbet bağlamı varsayılan olarak kapalıdır. Açıldığında yerel oturumdan
okunan bağlam seçilen backend'e gönderilebilir; hassas bilgileri göndermeden önce
bu ayarı kapatın. PromptPatch prompt veya telemetri kaydetmez.
