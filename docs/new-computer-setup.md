# Yeni Bilgisayarda PromptPatch

## Ön koşullar

- Codex CLI kurulu, giriş yapılmış ve `codex` komutu PATH içinde olmalı.

PromptPatch mevcut Codex girişini kullanır; Ollama, ayrı API anahtarı veya
PromptPatch sunucusu gerekmez.

## Linux ve macOS

```sh
curl -fsSL https://raw.githubusercontent.com/EyupEfeDuvarbasi/promptpatch/main/install.sh | sh
prompter --help
prompter setup-codex
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
prompter setup-codex
```

Yeni PowerShell açın ve `codex` komutunu çalıştırın. Profil wrapper'ı yüklenmezse:

```powershell
& "$env:LOCALAPPDATA\PromptPatch\bin\promptpatch-codex.cmd"
```

## Kontrol

```sh
codex login status
prompter --help
```

Codex içinde bir prompt yazıp `Ctrl-G` tuşuna basın. PromptPatch önce yalnızca
kararı etkileyen eksik bilgi varsa bir soru sorar, sonra iyileştirmeyi üretir.
Karşılaştırma ekranında seçim yapılana kadar dosyadaki özgün prompt değişmez.

Yakın sohbet bağlamını yeniden ayarlamak için:

```sh
prompter configure-context
```

Model çağrısı hata verirse özgün prompt korunur. Varsayılan 60 saniyelik editör
süresi içinde yanıt alınamıyorsa Codex oturumunu ve ağ bağlantısını kontrol edin.
