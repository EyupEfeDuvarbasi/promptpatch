# Yeni Bilgisayarda PromptPatch Kurulumu ve Testi

## Ön koşullar

- Codex CLI kurulu olmalı ve `codex` komutu PATH içinde çalışmalı.
- GitHub'dan kurulum için Go kurulu olmalı veya hazır PromptPatch binary'si kullanılmalı.
- Windows'ta PowerShell, Linux/macOS'ta Bash veya Zsh kullanılmalı.

PromptPatch Codex'i kurmaz; Codex'in `EDITOR`/`VISUAL` akışına bağlanır.

Windows'ta yerel model yapılandırması için depo kökündeki
scripti tek komutla çalıştırabilirsiniz:

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\install-promptpatch.ps1
```

Remote server ile:

```powershell
.\install-promptpatch.ps1 -ServerUrl "https://promptpatch.example.com" -ServerToken "uzun-rastgele-token"
```

Ollama'yı da kurmak için:

```powershell
.\install-promptpatch.ps1 -InstallOllama
```

Codex'i kontrol edin:

```text
codex --version
```

## Windows

Go yoksa PowerShell'de:

```powershell
winget install GoLang.Go
```

Yeni PowerShell açıp kontrol edin:

```powershell
go version
```

PromptPatch'i kurun:

```powershell
go install github.com/EyupEfeDuvarbasi/promptpatch/cmd/promptcheck@main
promptcheck --help
promptcheck setup-codex
```

Kurulum sorularında ilk test için:

1. Yakın sohbet bağlamı: `1` (kapalı)
2. Merkezi server: `N` (yerel Ollama)

Yeni PowerShell açın ve kontrol edin:

```powershell
Get-Command codex -All
```

İlk sonuç `Function codex` olmalıdır. Çalışmazsa launcher'ı doğrudan çalıştırın:

```powershell
& "$env:LOCALAPPDATA\PromptPatch\bin\promptpatch-codex.cmd"
```

## Linux/macOS

Go kurun, sonra:

```sh
go version
go install github.com/EyupEfeDuvarbasi/promptpatch/cmd/promptcheck@main
promptcheck --help
promptcheck setup-codex
```

İlk kurulumda yakın sohbet bağlamını kapalı bırakın ve merkezi server sorusuna
`N` yanıtı verin. Yeni terminal açın.

Kontrol:

```sh
command -v promptcheck
command -v codex
```

Kurulum shell'e göre `~/.bashrc` veya `~/.zshrc` dosyasına wrapper ekler.
Mevcut shell'i yenilemek için:

```sh
source ~/.bashrc
# veya
source ~/.zshrc
```

## İlk local test

Ollama kurmak ilk test için zorunlu değildir. Sistem şu sırayı kullanır:

1. Yapılandırılmış PromptPatch server.
2. Yerel Ollama.
3. Güvenilir çıktı üretilemezse özgün promptun korunması.

Codex'i açın, kısa bir prompt yazın ve `Ctrl-G` basın:

```text
şunu düzelt
```

Beklenen davranış:

- En fazla bir karar değiştirici soru sorulur.
- Sorular backend'den bağımsız olarak yerel eksik-bilgi kontrolüyle seçilir.
- Özgün ve iyileştirilmiş prompt gösterilir.
- `↑`/`↓` ile seçim yapılır.
- `Enter` seçilen promptu uygular.
- `Esc` özgün promptu korur.
- Ollama veya server yoksa özgün prompt korunur.

## Ollama ile local model testi

```sh
ollama pull qwen2.5:7b
curl http://127.0.0.1:11434/api/tags
```

Windows PowerShell:

```powershell
Invoke-RestMethod http://127.0.0.1:11434/api/tags
```

Model seçmek için:

```powershell
$env:PROMPTPATCH_OLLAMA_MODEL = "qwen2.5:7b"
codex
```

Linux/macOS:

```sh
export PROMPTPATCH_OLLAMA_MODEL=qwen2.5:7b
codex
```

Ollama'yı internete açmayın; `127.0.0.1:11434` üzerinde tutun.

## Remote server ile test

İstemci bilgisayarında Ollama olmadan çalışmak için:

PowerShell:

```powershell
$env:PROMPTPATCH_API_URL = "https://promptpatch.example.com"
$env:PROMPTPATCH_API_TOKEN = "uzun-rastgele-token"
codex
```

Linux/macOS:

```sh
export PROMPTPATCH_API_URL=https://promptpatch.example.com
export PROMPTPATCH_API_TOKEN=uzun-rastgele-token
codex
```

URL server kökü olmalıdır; sonuna `/v1/improve` eklemeyin. Token,
server'daki `PROMPTPATCH_SERVER_TOKEN` ile aynı olmalıdır.

Alternatif olarak `promptcheck setup-codex` sırasında merkezi server sorusuna
`y` yanıtlayıp URL'yi girin. Token config dosyasına yazılmaz; yukarıdaki
`PROMPTPATCH_API_TOKEN` ortam değişkeniyle verin. Sohbet bağlamı açıksa,
uzak server'a gönderim için ayrıca onay istenir.

Remote server erişilemezse istemci yerel Ollama'yı dener. Güvenilir bir çıktı
üretilemezse özgün promptu değiştirmez ve kalite hatasını gösterir.

## Bağlamı sonradan açma

İlk kurulumda bağlamı kapalı seçtiyseniz ayarı yeniden seçmek için:

```powershell
promptcheck configure-context
```

Bağlamı açtığınızda son 3.000 kelime kullanılır. PromptPatch mevcut proje
oturumundaki en yeni kullanıcı/yardımcı mesajlarını
referans olarak kullanır. Bu içerik seçilen remote server veya Ollama backend'ine
gönderilebilir.

## Doğrulama listesi

```text
codex --version
go version                    # go install kullanılıyorsa
promptcheck --help
promptcheck setup-codex
```

Son test: Codex'i açın, prompt yazın, `Ctrl-G` basın ve `Enter` veya `Esc` ile
akışı tamamlayın.

## Sorun giderme

### `codex` bulunamıyor

Codex kurulu değildir veya PATH içinde değildir. Windows'ta
`Get-Command codex -All`, Linux/macOS'ta `command -v codex` kullanın.

### `promptcheck` bulunamıyor

Go'nun binary klasörü PATH içinde değildir. `go env GOPATH` ve `go env GOBIN`
çıktılarını kontrol edip yeni terminal açın.

### `Ctrl-G` çalışmıyor

`promptcheck setup-codex` komutunu yeniden çalıştırın ve yeni terminal açın.
Windows'ta ilk `codex` sonucu `Function codex` olmalıdır.

### Ollama çalışmıyor

`curl http://127.0.0.1:11434/api/tags` veya PowerShell eşdeğerini çalıştırın.
`ollama list` ile modelin kurulu olduğunu kontrol edin. Qwen yoksa PromptPatch
kurulu `gemma3:4b` modelini kullanabilir.

### Remote server yetkisiz hatası

`PROMPTPATCH_API_TOKEN` ile server tokenının aynı olduğunu kontrol edin. Public
server bind adresinde token zorunludur.

### Özgün prompt korunuyor

Model çıktısı kesilmiş, somut gereksinimleri kaybetmiş veya kaynakta olmayan bilgi eklemiş olabilir.
Bu, güvenilir olmayan çıktının otomatik uygulanmasını önleyen beklenen güvenlik
davranışıdır.
