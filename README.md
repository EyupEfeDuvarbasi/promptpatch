# PromptPatch

PromptPatch, Codex içinde yazdığınız taslak promptu `Ctrl-G` ile açar, mevcut
Codex hesabınızı kullanarak iyileştirir ve özgün sürümle karşılaştırmanızı sağlar.
İyileştirilmiş prompt siz onaylamadan uygulanmaz.

## Kurulum

Gereksinimler: çalışan ve giriş yapılmış bir Codex CLI ile Go.

```sh
go install github.com/EyupEfeDuvarbasi/promptpatch/cmd/promptcheck@main
promptcheck setup-codex
```

Yeni bir terminal açıp `codex` komutunu çalıştırın. Prompt yazarken `Ctrl-G`
tuşuna basın; karşılaştırma ekranında özgün veya iyileştirilmiş sürümü seçin.

Yakın sohbet bağlamı ayarını sonradan değiştirmek için:

```sh
promptcheck configure-context
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
