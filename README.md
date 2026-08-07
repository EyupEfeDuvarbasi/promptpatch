# promptpatch

Terminalde AI kodlama promptlarındaki bilgi eksiklerini tespit eder; model erişimi olduğunda bunları gerçek puanlama ve iyileştirme için kullanır.

## Kurulum

```sh
go install github.com/EyupEfeDuvarbasi/promptpatch/cmd/promptcheck@main
```

Alternatif olarak GitHub Releases sayfasından platformunuza uygun binary'yi indirin ve PATH içindeki bir dizine koyun.

## Codex'te `Ctrl-G`

Codex CLI, `Ctrl-G` ile o an yazdığınız promptu editöre aktarır. Bir kez aşağıdaki kurulumu yapın ve **yeni bir terminalde Codex'i normal şekilde açın**:

```sh
promptcheck setup-codex
```

`Ctrl-G` artık aynı terminalde PromptPatch ekranını açar. Taslak prompt otomatik gelir, ekran yatay kaydırma gerektirmeden satır kırar; eksik bilgi varsa en fazla iki soru sorar. Model erişimi varsa yanıtları kullanarak gerçek iyileştirilmiş promptu ve iki puanı üretir; `↑`/`↓` ve `Enter` ile sürümü seçersiniz. Model erişimi yoksa özgün prompt korunur.

Bu ayar yalnızca `codex` komutunu saran bir shell fonksiyonu ekler; başka programların editör tercihini değiştirmez.

## Komut satırında kullanım

```sh
promptcheck "src/parser.go içindeki parseInput fonksiyonunu boş girdi için düzelt"
echo "şunu düzelt" | promptcheck
promptcheck --detail "prompt metni"
```

## İsteğe bağlı LLM modu

Varsayılan mod tamamen local çalışır ve API anahtarı istemez. Daha ayrıntılı anlamsal değerlendirme için `--model <isim>` kullanın; araç ilk kullanımda sağlayıcıyı ve gerekirse API anahtarını sorar.

Promptlar, telemetri ve geçmiş local modda kaydedilmez.
