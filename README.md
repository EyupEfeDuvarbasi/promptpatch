# PromptPatch

PromptPatch, Codex içinde yazdığınız taslak promptu `Ctrl-G` ile açar, mevcut
Codex hesabınızı kullanarak iyileştirir ve özgün sürümle karşılaştırmanızı sağlar.
İyileştirilmiş prompt siz onaylamadan uygulanmaz.

## Kurulum

Gereksinim: çalışan ve giriş yapılmış bir Codex CLI.

```sh
curl -fsSL https://raw.githubusercontent.com/EyupEfeDuvarbasi/promptpatch/main/install.sh | sh
```

Kurulum Codex entegrasyonunu ve yerel yardımcı servisi başlatır, ardından giriş
ekranını tarayıcıda açar. Yeni bir terminalde `codex` komutunu çalıştırın. Prompt yazarken `Ctrl-G`
tuşuna basın; karşılaştırma ekranında özgün veya iyileştirilmiş sürümü seçin.

Canlı arayüz yayına alındığında yardımcı uygulama adresi şu değişkenle verilir:

```sh
PROMPTER_WEB_URL=https://prompter.dev/setup prompter start
```

Yakın sohbet bağlamı ayarını sonradan değiştirmek için:

```sh
prompter configure-context
```

Windows'ta depo kökündeki kurulum betiği Go'yu gerekirse kurar ve aynı ayarı
yapar:

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\install-promptpatch.ps1
```

PromptPatch ayrı bir API anahtarı veya Ollama gerektirmez. Varsayılan model
`gpt-5.6-terra`, düşünme düzeyi `medium` değeridir. Gerektiğinde
`PROMPTPATCH_CODEX_MODEL` ve `PROMPTPATCH_CODEX_REASONING` ortam değişkenleriyle
değiştirilebilir.

## Doğrulama

```sh
go test ./...
go vet ./...
go run ./tools/prompttest
```

Gerçek model kalite karşılaştırması:

```sh
go run ./tools/promptbench -tier core -model gpt-5.6-terra -reasoning medium
```

Ayrıntılı platform adımları için [yeni bilgisayar kurulumu](docs/new-computer-setup.md)
belgesine bakın.

## Web arayüzü

Prompter çalışma alanını yerelde açmak için:

```sh
go run ./cmd/promptcheck serve
```

Ardından `http://127.0.0.1:8787` adresini açın. Arayüz binary içine gömülüdür;
ayrı bir frontend kurulumu gerekmez.

Release binary kurulumu (Linux/macOS):

```sh
curl -fsSL https://raw.githubusercontent.com/EyupEfeDuvarbasi/promptpatch/main/install.sh | sh
```

Teşhis ve yerel veri yaşam döngüsü:

```sh
prompter version
prompter doctor
prompter support-bundle
prompter data status
prompter data reset --all
prompter uninstall
```

Yerel alpha kurulum, test, sıfırlama ve hata raporlama adımları için
[alpha test rehberine](docs/alpha-testing.md) bakın.

### Google ve GitHub ile giriş

OAuth uygulamalarında callback adreslerini tanımlayın:

```text
http://127.0.0.1:8787/auth/google/callback
http://127.0.0.1:8787/auth/github/callback
```

Sağlayıcı bilgileriyle çalıştırın:

```sh
GOOGLE_CLIENT_ID=... \
GITHUB_CLIENT_ID=... GITHUB_CLIENT_SECRET=... \
PROMPTER_SESSION_SECRET='en-az-32-karakter-rastgele-bir-deger' \
go run ./cmd/promptcheck serve
```

Canlı ortamda `PROMPTER_PUBLIC_URL=https://uygulama.example.com` verilmelidir.
HTTP public URL veya kısa session secret ile OAuth başlatılmaz.

Google için **Desktop app** client kullanılmalıdır; PKCE akışında client secret
zorunlu değildir. GitHub secret binary içine gömülmez; özel alpha testinde
yalnız ortam değişkeniyle verilmeli ve test sonrasında döndürülmelidir.
