# House Chores — domowe obowiązki w Home Assistant

[🇬🇧 English](README.md) · 🇵🇱 Polski

Appka Home Assistanta (dawniej add-on) do dzielenia się domowymi obowiązkami. Przypomnienia przychodzą jako powiadomienia z przyciskami z aplikacji Home Assistant (Companion) na Androidzie, a interfejs otwiera się wewnątrz Home Assistanta przez ingress, bez osobnej aplikacji i logowania.

> **Stan:** wczesny etap (kamień milowy M0, szkielet). Appka się instaluje, wysyła testowe powiadomienie z przyciskami i odbiera kliknięcia. Obowiązki, harmonogramy i statystyki dojdą w kolejnych etapach.

## Funkcje (planowane)

- Zadania na konkretny dzień, tygodniowe, bez terminu („trzeba zrobić” / „zrobione”) oraz wyzwalane przez urządzenia (pralka, suszarka, zmywarka) na podstawie stanu encji albo poboru mocy.
- Przypomnienia tylko w oknie powiadomień (domyślnie 10:00–20:00), ponawiane do odhaczenia.
- Kalendarz i porównanie, kto co zrobił, z wagami zadań.
- Eksport miesiąca dla dowolnego LLM i import proponowanych wag.
- Interfejs i powiadomienia po polsku i po angielsku, zgodnie z językiem telefonu lub przeglądarki. Kolejne języki dodaje się w jednym pliku (`house_chores/internal/i18n`).

## Instalacja

1. W Home Assistancie: **Ustawienia → Apps → App store**, otwórz okno repozytoriów i dodaj:
   ```
   https://github.com/Fryk/house-chores-ha
   ```
2. Zainstaluj **House Chores** i uruchom. Pierwsze uruchomienie buduje obraz na urządzeniu, co trwa kilka minut.
3. Otwórz panel **House Chores** w menu bocznym. Nazwy panelu Home Assistant nie tłumaczy, ale interfejs w środku jest po polsku, jeśli telefon lub przeglądarka ma ustawiony polski.

Szczegóły: [house_chores/DOCS.md](house_chores/DOCS.md).

## Rozwój

Appka to jedna binarka Go z SQLite (`modernc.org/sqlite`, bez CGO) i HTML renderowanym po stronie serwera. Bez zmiennej `SUPERVISOR_TOKEN` działa w **trybie fake**: powiadomienia trafiają do logu i na stronę `/dev`, gdzie można klikać ich przyciski.

```sh
cd house_chores
go test ./...
DATA_DIR=./data PORT=8080 go run ./cmd/house-chores
# otwórz http://localhost:8080 i http://localhost:8080/dev
```

Wymaga Go 1.26 lub nowszego. Specyfikacja projektu: [SPEC.md](SPEC.md).

## Układ repozytorium

```text
repository.yaml   metadane repozytorium appek Home Assistanta
house_chores/     appka (nazwa katalogu = slug)
  config.yaml     konfiguracja appki
  Dockerfile
  cmd/, internal/ kod Go
  web/            osadzone szablony i pliki statyczne
SPEC.md           specyfikacja
```

## Licencja

[MIT](LICENSE) © 2026 Paweł Bełza
