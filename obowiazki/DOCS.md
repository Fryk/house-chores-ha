# Obowiązki — dokumentacja

## Instalacja

1. Ustawienia → Apps → App store → dodaj repozytorium `https://github.com/Fryk/house-chores-ha`.
2. Zainstaluj appkę „Obowiązki” i uruchom ją.
3. Panel „Obowiązki” pojawi się w menu bocznym Home Assistanta.

## Test powiadomień (M0)

Na stronie głównej appki wybierz usługę `notify.mobile_app_*` swojego telefonu i kliknij „Wyślij testowe powiadomienie”.
Kliknięcie przycisku w powiadomieniu pojawi się w logu appki i na liście zdarzeń na stronie.

## Uruchomienie lokalne (tryb fake)

Bez zmiennej `SUPERVISOR_TOKEN` appka działa w trybie fake: powiadomienia trafiają do logu i na stronę `/dev`.

```sh
cd obowiazki
DATA_DIR=./data PORT=8080 go run ./cmd/obowiazki
# albo
docker build -t obowiazki . && docker run --rm -p 8080:8080 -e PORT=8080 obowiazki
```
