# Prompter Yerel Alpha Testi

Her testçi Prompter'ı kendi bilgisayarında çalıştırır. Proje yolları, hesaplar ve
Codex promptları cihazdan çıkmaz.

## Kurulum

Release sayfasından işletim sisteminize uygun arşivi indirin veya kurucuyu
çalıştırın. Ardından:

```sh
prompter setup-codex
prompter serve
```

Tarayıcıda `http://127.0.0.1:8787` adresini açın, yerel hesap oluşturun ve
**Projeler > Yeni proje** ile bir klasör seçin.

## Test senaryoları

1. Hesap oluşturun, çıkış yapın, doğru ve yanlış parolayla giriş deneyin.
2. Git projesi, Git dışı klasör ve boş bir proje ekleyin.
3. Aynı projeyi ikinci kez ekleyin; kopya oluşmadığını doğrulayın.
4. Prompt arayın, proje filtresini değiştirin ve prompt detayını açın.
5. CSV ve JSON dışa aktarmayı deneyin.
6. Sunucuyu yeniden başlatın; hesabın ve projelerin korunduğunu doğrulayın.
7. Türkçe/İngilizce geçişinde çevrilmemiş metin arayın.
8. `prompter doctor` çalıştırın.

## Temiz başlangıç

```sh
prompter data status
prompter data reset --all
```

Reset yalnız Prompter verisini siler; Codex oturumlarına veya proje dosyalarına
dokunmaz.

## Hata raporu

Rapora işletim sistemi, Prompter sürümü, tekrar adımları ve beklenen/gerçek
davranışı ekleyin. İçeriksiz teşhis paketi:

```sh
prompter support-bundle
```

Paket prompt, e-posta, token, cookie veya dosya yolu içermez.
