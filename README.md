# promptpatch

Terminalde AI kodlama promptlarını API anahtarı olmadan puanlar, eksik bilgi varsa en fazla iki soru sorar ve iyileştirilmiş sürümünü üretir.

## Kurulum

```sh
go install github.com/EyupEfeDuvarbasi/promptpatch@v0.1.0
```

Alternatif olarak GitHub Releases sayfasından platformunuza uygun binary'yi indirin ve PATH içindeki bir dizine koyun.

## Kullanım

```sh
promptcheck "src/parser.go içindeki parseInput fonksiyonunu boş girdi için düzelt"
echo "şunu düzelt" | promptcheck
promptcheck --detail "prompt metni"
```

Shell kısayolunu bir kez kurun:

```sh
promptcheck setup-shell
```

Yeni terminal açtıktan sonra shell komut satırındaki prompt metnini `Ctrl-G` ile değerlendirebilirsiniz. Kısayol zsh ve bash içindir.

## İsteğe bağlı LLM modu

Varsayılan mod tamamen local çalışır ve API anahtarı istemez. Daha ayrıntılı anlamsal değerlendirme için `--model <isim>` kullanın; araç ilk kullanımda sağlayıcıyı ve gerekirse API anahtarını sorar.

Promptlar, telemetri ve geçmiş local modda kaydedilmez.
