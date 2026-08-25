# Yeni Bilgisayarda PromptPatch

## Ön koşullar

- Codex CLI kurulu, giriş yapılmış ve `codex` komutu PATH içinde olmalı.
- Kaynaktan kurulum için Go kurulu olmalı.

PromptPatch mevcut Codex girişini kullanır; Ollama, ayrı API anahtarı veya
PromptPatch sunucusu gerekmez.

## Linux ve macOS

```sh
go install github.com/EyupEfeDuvarbasi/promptpatch/cmd/promptcheck@main
promptcheck --help
promptcheck setup-codex
```

Kurulum `~/.zshrc` veya `~/.bashrc` içine Codex wrapper'ını ekler. Yeni terminal
açın, `codex` komutunu çalıştırın ve taslak prompt üzerinde `Ctrl-G` kullanın.

## Windows

Depoyu indirdiyseniz PowerShell'de:

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\install-promptpatch.ps1
```

Elle kurulum:

```powershell
winget install GoLang.Go
go install github.com/EyupEfeDuvarbasi/promptpatch/cmd/promptcheck@main
promptcheck setup-codex
```

Yeni PowerShell açın ve `codex` komutunu çalıştırın. Profil wrapper'ı yüklenmezse:

```powershell
& "$env:LOCALAPPDATA\PromptPatch\bin\promptpatch-codex.cmd"
```

## Kontrol

```sh
codex login status
promptcheck --help
```

Codex içinde bir prompt yazıp `Ctrl-G` tuşuna basın. PromptPatch önce yalnızca
kararı etkileyen eksik bilgi varsa bir soru sorar, sonra iyileştirmeyi üretir.
Karşılaştırma ekranında seçim yapılana kadar dosyadaki özgün prompt değişmez.

Yakın sohbet bağlamını yeniden ayarlamak için:

```sh
promptcheck configure-context
```

Model çağrısı hata verirse özgün prompt korunur. Varsayılan 60 saniyelik editör
süresi içinde yanıt alınamıyorsa Codex oturumunu ve ağ bağlantısını kontrol edin.
