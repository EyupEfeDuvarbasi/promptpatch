# promptpatch

Terminalde AI kodlama promptlarındaki bilgi eksiklerini yerel kurallarla puanlar ve yerel modelle iyileştirilmiş geliştirici promptu üretir.

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

`Ctrl-G` artık aynı terminalde PromptPatch ekranını açar. Taslak prompt otomatik gelir, ekran yatay kaydırma gerektirmeden satır kırar; eksik bilgi varsa en fazla iki soru sorar. İlk kurulumda yakın sohbet bağlamı için kapalı, 800, 2.000 veya 4.000 kelimelik sınır seçilir (varsayılan 2.000). Eşleşen yerel oturum bulunursa yalnızca en yeni tam kullanıcı/yardımcı mesajları referans olarak kullanılır; bulunamazsa taslak tek başına işlenir.

Yanıtlar yerel modele verilir; model gerçek iyileştirilmiş promptu üretir, iki sürüm yerel kurallarla puanlanır. Soru-cevap metnini sona ekleyen veya somut gereksinimleri kaybeden model çıktıları kabul edilmez. `↑`/`↓` ve `Enter` ile sürümü seçersiniz.

Bu ayar yalnızca `codex` komutunu saran bir shell fonksiyonu ekler; başka programların editör tercihini değiştirmez.

## Komut satırında kullanım

```sh
promptcheck "src/parser.go içindeki parseInput fonksiyonunu boş girdi için düzelt"
echo "şunu düzelt" | promptcheck
promptcheck --detail "prompt metni"
```

## İsteğe bağlı LLM modu

Varsayılan mod tamamen local çalışır ve API anahtarı istemez. Daha ayrıntılı anlamsal değerlendirme için `--model <isim>` kullanın; araç ilk kullanımda sağlayıcıyı ve gerekirse API anahtarını sorar.

PromptPatch prompt veya telemetri kaydetmez. Yakın bağlam etkinse CLI’nin zaten yerelde tuttuğu oturum kaydını yalnızca bellek içinde okur.
