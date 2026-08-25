# Model iyileştirme korpusu

`model-cases.jsonl`, PromptPatch'in model tabanlı yeniden yazma kalitesini ve
kullanım maliyetini karşılaştırmak içindir. Mevcut `cases.jsonl` puanlama ve
netleştirme sorularını test eder; bu dosyadaki girdiler ise doğrudan yeniden
yazılabilecek kadar bilgi içerir.

Korpus 20 görev alanında üçer vaka içerir: bir `core` karşılaştırma vakası ve
iki genişletilmiş vaka. Böylece ilk model elemesi 20 × 3 yapılandırma çağrısıyla
yapılabilir; yalnızca kazanan yapılandırma 60 vakanın tamamında çalıştırılır.

Kapsanan görev alanları:

- hata düzeltme, backend özellik, frontend/erişilebilirlik ve mobil;
- API entegrasyonu, veritabanı/SQL, performans ve eşzamanlılık;
- güvenlik, gizlilik, CI/CD, container/Kubernetes/Terraform;
- incident/observability, test, refactor/bağımlılık yükseltme;
- mimari/araştırma/plan, veri/ML, embedded/edge ve doküman/CLI/içerik.

Kapsanan biçim ve risk eksenleri:

- Türkçe, İngilizce, karışık dil, yazım hatalı ve konuşma bağlamına bağlı giriş;
- kısa, uzun, yapılandırılmış, log/kanıt içeren ve zaten güçlü prompt;
- kesin sayı/kimlik koruma, olumsuz kısıt, işlem sırası ve çıktı biçimi;
- belirsizliği uydurmama, geriye uyumluluk, veri kaybı, yetkilendirme,
  gizlilik, onay sınırı, platform ve kaynak bütçesi.

Değerlendirme sırası:

1. Boş/yarım çıktı, kaybolan kesin terim, kaynakta olmayan sayı ve yasaklanan
   ekleme gibi kırıcı hatalar geçiş kapısıdır.
2. Geçen çıktılarda niyet koruma, açıklık, uygulanabilirlik ve sadelik
   değerlendirilir.
3. Kalite tabanını geçen yapılandırmalar kullanım tokenları ve gecikmeyle
   karşılaştırılır; düşük maliyet kötü kaliteyi telafi etmez.
4. Gerçek kullanıcı hataları daha sonra aynı etiketlerle korpusa eklenir.
